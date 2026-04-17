package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bloomerab/convoy/config"
	"github.com/bloomerab/convoy/internal/client"
	"github.com/bloomerab/convoy/internal/dao"
	"github.com/bloomerab/convoy/internal/model"
	"github.com/bloomerab/convoy/internal/view"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const refreshInterval = 2 * time.Second

// App is the main application.
type App struct {
	tviewApp  *tview.Application
	cfg       config.Config
	factory   *client.ClusterFactory
	pageStack *PageStack
	header    *view.Header
	footer    *view.Footer
	dashboard *view.Dashboard
	cmdInput  *view.CmdBar
	layout    *tview.Flex
	watchers  []dao.Watcher
	cancel    context.CancelFunc
}

func New(cfg config.Config) *App {
	return &App{
		tviewApp: tview.NewApplication(),
		cfg:      cfg,
		factory:  client.NewClusterFactory(),
	}
}

func (a *App) Init() error {
	clusters := a.cfg.Clusters
	if len(clusters) == 0 {
		discovered, err := config.DiscoverClusters()
		if err != nil {
			return fmt.Errorf("discover clusters: %w", err)
		}
		clusters = discovered
	}

	if len(clusters) == 0 {
		return fmt.Errorf("no clusters configured or discovered")
	}

	for _, c := range clusters {
		if err := a.factory.AddCluster(c); err != nil {
			log.Printf("WARN: skipping cluster %s: %v", c.Name, err)
			continue
		}
	}

	if len(a.factory.Clients()) == 0 {
		return fmt.Errorf("no clusters reachable")
	}

	a.header = view.NewHeader()
	a.footer = view.NewFooter()
	a.dashboard = view.NewDashboard(a.onDescribe)
	a.pageStack = NewPageStack()
	a.pageStack.Push("dashboard", a.dashboard)
	a.cmdInput = view.NewCmdBar(a.onCommand, a.onCmdCancel)

	a.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.pageStack, 0, 1, true).
		AddItem(a.footer, 1, 0, false)

	a.tviewApp.SetRoot(a.layout, true)
	a.tviewApp.SetInputCapture(a.handleInput)

	return nil
}

func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.startWatchers(ctx)
	go a.refreshLoop(ctx)
	return a.tviewApp.Run()
}

func (a *App) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	a.tviewApp.Stop()
}

func (a *App) startWatchers(ctx context.Context) {
	for _, cc := range a.factory.Clients() {
		watchers := []dao.Watcher{
			dao.NewKustomizationDAO(cc, func() {}),
			dao.NewHelmReleaseDAO(cc, func() {}),
			dao.NewGitRepositoryDAO(cc, func() {}),
		}
		for _, w := range watchers {
			a.watchers = append(a.watchers, w)
			go func(w dao.Watcher) {
				if err := w.Start(ctx); err != nil && ctx.Err() == nil {
					log.Printf("watcher error: %v", err)
				}
			}(w)
		}
	}
}

// refreshLoop redraws the UI on a fixed interval — watchers just populate the cache.
func (a *App) refreshLoop(ctx context.Context) {
	// Initial draw after a brief delay to let watchers populate
	time.Sleep(1 * time.Second)
	a.redraw()

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.redraw()
		}
	}
}

func (a *App) redraw() {
	var all []model.Resource
	for _, w := range a.watchers {
		all = append(all, w.Resources()...)
	}

	a.tviewApp.QueueUpdateDraw(func() {
		a.dashboard.Refresh(all)
		a.header.Update(all, len(a.factory.Clients()))
	})
}

func (a *App) onDescribe(r model.Resource) {
	dv := view.NewDescribeView(r)
	a.pageStack.Push("describe", dv)
}

func (a *App) handleInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlC:
		a.Stop()
		return nil
	case tcell.KeyEscape:
		a.pageStack.Pop()
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case 'q':
			a.Stop()
			return nil
		case 'r':
			a.redraw()
			return nil
		case ':':
			a.showCmdBar(":")
			return nil
		case '/':
			a.showCmdBar("/")
			return nil
		}
	}
	return event
}

func (a *App) showCmdBar(prefix string) {
	a.cmdInput.Activate(prefix)
	a.layout.RemoveItem(a.footer)
	a.layout.AddItem(a.cmdInput, 1, 0, true)
	a.tviewApp.SetFocus(a.cmdInput)
}

func (a *App) hideCmdBar() {
	a.layout.RemoveItem(a.cmdInput)
	a.layout.AddItem(a.footer, 1, 0, false)
	a.tviewApp.SetFocus(a.pageStack)
}

func (a *App) onCmdCancel() {
	a.hideCmdBar()
}

func (a *App) onCommand(text string) {
	a.hideCmdBar()

	cmd := ParseCommand(text)
	switch cmd.Name {
	case "config":
		a.execConfig()
	case "q", "quit":
		a.Stop()
	}
}

func (a *App) execConfig() {
	files := view.DiscoverConfigFiles()
	cl := view.NewConfigListView(files, a.onConfigSelect, a.onConfigEdit)
	a.pageStack.Push("config", cl)
}

func (a *App) onConfigSelect(f view.ConfigFile) {
	detail := view.NewConfigDetailView(f, a.onConfigEdit)
	a.pageStack.Push("config-detail", detail)
}

func (a *App) onConfigEdit(f view.ConfigFile) {
	newCfg, changed, err := editConfigAndReload(a.tviewApp, f.Path)
	if err != nil {
		log.Printf("config edit error: %v", err)
		return
	}
	if changed {
		a.cfg = newCfg
		a.restartWatchers()
	}
	if a.pageStack.Current() == "config-detail" {
		a.pageStack.Pop()
		a.onConfigSelect(f)
	}
}

func (a *App) restartWatchers() {
	if a.cancel != nil {
		a.cancel()
	}

	a.factory = client.NewClusterFactory()
	a.watchers = nil

	clusters := a.cfg.Clusters
	if len(clusters) == 0 {
		discovered, err := config.DiscoverClusters()
		if err != nil {
			log.Printf("discover clusters: %v", err)
			return
		}
		clusters = discovered
	}

	for _, c := range clusters {
		if err := a.factory.AddCluster(c); err != nil {
			log.Printf("WARN: skipping cluster %s: %v", c.Name, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.startWatchers(ctx)
	go a.refreshLoop(ctx)
}
