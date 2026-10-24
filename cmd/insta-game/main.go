package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fogo-sh/insta-game/internal/command"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	args := os.Args[1:]
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		args = []string{"lambda"}
	}
	if err := command.Run(ctx, args, os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
