package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/isureal-company/isureal"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config := isureal.ConfigFromEnv()
	listeners, err := config.RegionListeners()
	if err != nil {
		return err
	}
	if config.TracingEnabled {
		region := config.Region
		if len(listeners) > 1 {
			region = "multi"
		}
		shutdown, err := isureal.StartTracing(context.Background(), region)
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
	runtime := isureal.NewRuntime(config)
	defer runtime.Close()
	var servers []*http.Server
	var sockets []net.Listener
	for _, listener := range listeners {
		regional := *config
		regional.Region = listener.Region
		server := &http.Server{Addr: listener.Address, Handler: isureal.NewRegionServer(&regional, runtime), ReadHeaderTimeout: 10 * time.Second}
		socket, err := net.Listen("tcp", listener.Address)
		if err != nil {
			for _, s := range sockets {
				_ = s.Close()
			}
			return err
		}
		sockets = append(sockets, socket)
		servers = append(servers, server)
	}
	if config.DiagnosticsAddr != "" {
		server, socket, err := isureal.NewDiagnosticsServer(config.DiagnosticsAddr, runtime.DB.DB)
		if err != nil {
			for _, s := range sockets {
				_ = s.Close()
			}
			return err
		}
		servers = append(servers, server)
		sockets = append(sockets, socket)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failures := make(chan error, len(servers))
	for i, server := range servers {
		go func(s *http.Server, l net.Listener) { failures <- s.Serve(l) }(server, sockets[i])
	}
	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-failures:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, server := range servers {
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
	}
	return serveErr
}
