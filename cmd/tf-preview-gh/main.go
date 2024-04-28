package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nimbolus/terraform-backend/pkg/speculative"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := speculative.Run(ctx); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
