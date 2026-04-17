package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bloomerab/convoy/config"
	"github.com/rivo/tview"
)

// editFile suspends the TUI and opens the given path in $EDITOR.
func editFile(tviewApp *tview.Application, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}

	// Create with defaults if it doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := config.DefaultConfig()
		if err := config.WriteToPath(path, cfg); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vim"
	}

	tviewApp.Suspend(func() {
		cmd := exec.Command(editor, path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	})

	return nil
}

// editConfigAndReload opens the main config in $EDITOR, reloads, and reports if changed.
func editConfigAndReload(tviewApp *tview.Application, path string) (config.Config, bool, error) {
	oldCfg, _ := config.Load()

	if err := editFile(tviewApp, path); err != nil {
		return oldCfg, false, err
	}

	newCfg, err := config.Load()
	if err != nil {
		return oldCfg, false, fmt.Errorf("reload config: %w", err)
	}

	changed := !configEqual(oldCfg, newCfg)
	return newCfg, changed, nil
}

func configEqual(a, b config.Config) bool {
	if a.RawRefresh != b.RawRefresh {
		return false
	}
	if a.GitHub.Org != b.GitHub.Org || a.GitHub.Token != b.GitHub.Token || a.GitHub.MaxRuns != b.GitHub.MaxRuns {
		return false
	}
	if len(a.GitHub.Repos) != len(b.GitHub.Repos) {
		return false
	}
	for i := range a.GitHub.Repos {
		if a.GitHub.Repos[i] != b.GitHub.Repos[i] {
			return false
		}
	}
	if len(a.Clusters) != len(b.Clusters) {
		return false
	}
	for i := range a.Clusters {
		if a.Clusters[i] != b.Clusters[i] {
			return false
		}
	}
	return true
}
