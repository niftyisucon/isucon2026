package isureal

import (
	"database/sql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// Keep the existing concurrency limit and reuse more idle connections.
const maxDBConnections = 30
const maxIdleDBConnections = 10

func configureDatabasePool(db *sql.DB) {
	db.SetMaxOpenConns(maxDBConnections)
	db.SetMaxIdleConns(maxIdleDBConnections)
}

type HandlerContext struct {
	DB            *sqlx.DB
	Config        *Config
	IPBans        *IPBanList
	Images        *ImageCache
	Files         *ImageStore
	StorageColumn bool
	Notifications *notificationPoller
}

func NewServer(config *Config) *echo.Echo {
	return NewRegionServer(config, NewRuntime(config))
}

// Runtime is shared by region listeners on asia1. No authorization results are cached.
type Runtime struct {
	DB            *sqlx.DB
	IPBans        *IPBanList
	Images        *ImageCache
	Files         *ImageStore
	StorageColumn bool
	Notifications *notificationPoller
}

func NewRuntime(config *Config) *Runtime {
	db, err := openDatabase(config)
	if err != nil {
		panic("failed to open database: " + err.Error())
	}
	configureDatabasePool(db.DB)
	err = db.Ping()
	if err != nil {
		panic("failed to ping database: " + err.Error())
	}

	r := &Runtime{DB: db, IPBans: &IPBanList{}, Images: NewImageCache()}
	if err = db.Get(&r.StorageColumn, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='image_objects' AND column_name='storage_key')`); err != nil {
		panic(err)
	}
	if config.ImageDir != "" {
		if _, err = db.Exec("SELECT storage_key FROM image_objects LIMIT 0"); err != nil {
			panic("run architecture.sql before enabling image files")
		}
		r.Files, err = NewImageStore(config.ImageDir)
		if err != nil {
			panic(err)
		}
	}
	r.Notifications = newNotificationPoller(db)
	r.Notifications.onReset = r.Images.Reset
	var eventSchema bool
	if err = db.Get(&eventSchema, `SELECT count(*)=3 FROM pg_trigger WHERE tgname IN ('isureal_notification_changed','isureal_window_changed','isureal_user_changed') AND tgrelid IN ('notifications'::regclass,'posting_windows'::regclass,'users'::regclass) AND tgenabled='O'`); err != nil {
		panic(err)
	}
	// A channel can be LISTENed to before its writers exist. Do not relax the
	// 500ms fallback until all transactional notification triggers are installed.
	if eventSchema {
		if err = db.Get(&r.Notifications.epochSchema, `SELECT to_regclass('isureal_runtime_state') IS NOT NULL`); err != nil {
			panic(err)
		}
		r.Notifications.startListener(config.DatabaseDSN())
	}
	return r
}

func (r *Runtime) Close() {
	r.Notifications.close()
	_ = r.DB.Close()
}

func NewRegionServer(config *Config, r *Runtime) *echo.Echo {
	if !validRegion(config.Region) {
		panic("invalid server region")
	}
	e := echo.New()
	e.IPExtractor = extractClientIP
	if config.TracingEnabled {
		e.Use(TracingMiddleware())
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c *echo.Context) error {
				trace.SpanFromContext(c.Request().Context()).SetAttributes(attribute.String("isureal.region", config.Region))
				return next(c)
			}
		})
	}
	e.Use(ActionLogMiddleware(config.LogLevel, os.Stdout))
	e.Use(middleware.Recover())
	e.HTTPErrorHandler = APIErrorHandler
	handlerContext := &HandlerContext{DB: r.DB, Config: config, IPBans: r.IPBans, Images: r.Images, Files: r.Files, StorageColumn: r.StorageColumn, Notifications: r.Notifications}
	if config.EdgeUpstream != "" {
		e.GET("/api/notifications", getNotificationsHandler(handlerContext), IPBanMiddleware(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
		e.Any("/api/*", newEdgeProxy(config, r.Images))
		return e
	}
	e.Use(IPBanMiddleware(handlerContext))

	e.POST("/api/initialize", initializeHandler(handlerContext))
	e.POST("/api/auth/register", registerHandler(handlerContext))
	e.POST("/api/auth/login", loginHandler(handlerContext))
	e.POST("/api/auth/logout", logoutHandler(handlerContext), SessionMiddleware(handlerContext, true))
	e.GET("/api/users", searchUsersHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/users/:userId", getUserHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/me/export", exportPostsHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/me", getMeHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.PATCH("/api/me", updateMeHandler(handlerContext), SessionMiddleware(handlerContext, true))
	e.PUT("/api/me/profile-image", putProfileImageHandler(handlerContext), SessionMiddleware(handlerContext, true))
	e.DELETE("/api/me/profile-image", deleteProfileImageHandler(handlerContext), SessionMiddleware(handlerContext, true))
	e.POST("/api/me/account-deletion", deleteAccountHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.POST("/api/posts", createPostHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/posting-window/current", getCurrentPostingWindowHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/posts/me/current", getMyCurrentPostHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/posts/:postId", getPostHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.PATCH("/api/posts/:postId", updatePostHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.DELETE("/api/posts/:postId", deletePostHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/feeds/:feed", getFeedHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/memories", getMemoriesHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/memories/:date", getMemoriesByDateHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.POST("/api/posts/:postId/comments", createCommentHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/posts/:postId/comments", getCommentsHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.DELETE("/api/comments/:commentId", deleteCommentHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.POST("/api/posts/:postId/realmojis", replaceRealMojiHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/posts/:postId/realmojis", getRealMojisHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.DELETE("/api/realmojis/:realMojiId", deleteRealMojiHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/images/:imageId", getImageHandler(handlerContext))
	e.GET("/api/notifications", getNotificationsHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.POST("/api/friend-requests", createFriendRequestHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.POST("/api/friend-requests/:requestId/:action", transitionFriendRequestHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.GET("/api/relationships/:kind", getRelationshipsHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.DELETE("/api/friendships/:userId", deleteFriendshipHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.PUT("/api/follows/:userId", followHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.DELETE("/api/follows/:userId", unfollowHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.PUT("/api/blocks/:userId", blockHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.DELETE("/api/blocks/:userId", unblockHandler(handlerContext), SessionMiddleware(handlerContext, true), RegionMiddleware(handlerContext))
	e.POST("/api/admin/auth/login", adminLoginHandler(handlerContext))
	e.POST("/api/admin/auth/logout", adminLogoutHandler(handlerContext), AdminSessionMiddleware(handlerContext, true))
	e.PUT("/api/admin/users/:userId/certification", certifyUserHandler(handlerContext), AdminSessionMiddleware(handlerContext, true))
	e.DELETE("/api/admin/users/:userId/certification", deleteUserCertificationHandler(handlerContext), AdminSessionMiddleware(handlerContext, true))
	e.POST("/api/admin/posting-window", openPostingWindowHandler(handlerContext), AdminSessionMiddleware(handlerContext, true))

	return e
}
