package utils

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func SetUpContext() context.Context {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ch
		cancel()
		<-ch
		os.Exit(1)
	}()
	return ctx
}
