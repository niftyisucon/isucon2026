package main

import (
	"context"
	"log"
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
	if err := server.Start(":8080"); err != nil {
		panic("failed to start server: " + err.Error())
	}
}
