package app

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/bloomerab/convoy/config"
	"github.com/rivo/tview"
)

// editConfig suspends the TUI, opens the config in $EDITOR, then reloads.
func editConfig(tviewApp *tview.Application) (config.Config, bool, error) {
	path, err := config.EnsureExists()
	if err != nil {
		return config.Config{}, false, fmt.Errorf("ensure config: %w", err)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vim"
	}

	oldCfg, _ := config.Load()

	tviewApp.Suspend(func() {
		cmd := exec.Command(editor, path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	})

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
