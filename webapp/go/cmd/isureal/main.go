package main

import (
	"context"
	"log"
	"os"
	"time"

	"example.com/isureal-company/isureal"
)

// 2026-09-23

func main() {
	config := isureal.ConfigFromEnv()
	if config.TracingEnabled {
		shutdown, err := isureal.StartTracing(context.Background(), config.Region)
		if err != nil {
			log.Print("tracing initialization failed; starting without tracing")
			config.TracingEnabled = false
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = shutdown(ctx)
			}()
		}
	}
	server := isureal.NewServer(config)
	// europe/america 用のアプリを asia 側に別ポートで立てるため、待ち受けポートを変えられるようにする。
	port := os.Getenv("ISUREAL_PORT")
	if port == "" {
		port = "8080"
	}
	if err := server.Start(":" + port); err != nil {
		panic("failed to start server: " + err.Error())
	}
}
