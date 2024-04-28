package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/nimbolus/terraform-backend/pkg/speculative"
)

func main() {
	rootCmd := speculative.NewCommand()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}
