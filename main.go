package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bloomerab/convoy/config"
	"github.com/bloomerab/convoy/internal/app"
)

func main() {
	// Log to file so we can debug TUI hangs
	logDir := config.ConfigDir()
	_ = os.MkdirAll(logDir, 0o755)
	logFile, err := os.OpenFile(filepath.Join(logDir, "convoy.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

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
