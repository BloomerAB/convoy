package main

import (
	"fmt"
	"os"

	"github.com/bloomerab/convoy/config"
	"github.com/bloomerab/convoy/internal/app"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	a := app.New(cfg)
	if err := a.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}

	if err := a.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
