package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/app"
	"github.com/pengmd/wechat-proxy/cli/get-doubao-pic/internal/config"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(config.ExitUsage)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	result, err := app.Run(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(app.ExitCode(err))
	}

	fmt.Printf("saved %d image(s) to %s\n", len(result.Files), result.OutputDir)
	for _, file := range result.Files {
		fmt.Println(file)
	}

	if len(result.Warnings) > 0 {
		fmt.Println("warnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("- %s\n", warning)
		}
	}

	if errors.Is(result.WarningErr, app.ErrPartialDownload) {
		os.Exit(config.ExitPartial)
	}
}
