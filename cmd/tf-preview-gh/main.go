package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/nimbolus/terraform-backend/pkg/scaffold"
	"github.com/nimbolus/terraform-backend/pkg/speculative"
)

func main() {
	rootCmd := speculative.NewCommand()
	rootCmd.AddCommand(scaffold.NewCommand())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}
