package app

import (
	"context"
	"fmt"
	"log"

	"github.com/bloomerab/convoy/config"
	"github.com/bloomerab/convoy/internal/client"
	"github.com/bloomerab/convoy/internal/dao"
	"github.com/bloomerab/convoy/internal/model"
	"github.com/bloomerab/convoy/internal/view"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// App is the main application.
type App struct {
	tviewApp   *tview.Application
	cfg        config.Config
	factory    *client.ClusterFactory
	tableModel *model.TableModel
	pageStack  *PageStack
	header     *view.Header
	footer     *view.Footer
	dashboard  *view.Dashboard
	watchers   []dao.Watcher
	cancel     context.CancelFunc
}

func New(cfg config.Config) *App {
	return &App{
		tviewApp:   tview.NewApplication(),
		cfg:        cfg,
		factory:    client.NewClusterFactory(),
		tableModel: model.NewTableModel(),
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
	a.dashboard = view.NewDashboard(a.tableModel)
	a.pageStack = NewPageStack()
	a.pageStack.Push("dashboard", a.dashboard)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.pageStack, 0, 1, true).
		AddItem(a.footer, 1, 0, false)

	a.tviewApp.SetRoot(layout, true)
	a.tviewApp.SetInputCapture(a.handleInput)

	return nil
}

func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	// Create watchers for each cluster
	for _, cc := range a.factory.Clients() {
		w := dao.NewKustomizationDAO(cc, a.onWatchUpdate)
		a.watchers = append(a.watchers, w)
		go func(w dao.Watcher) {
			if err := w.Start(ctx); err != nil && ctx.Err() == nil {
				log.Printf("watcher error: %v", err)
			}
		}(w)
	}

	return a.tviewApp.Run()
}

func (a *App) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	a.tviewApp.Stop()
}

func (a *App) onWatchUpdate() {
	var all []model.Resource
	for _, w := range a.watchers {
		all = append(all, w.Resources()...)
	}

	a.tviewApp.QueueUpdateDraw(func() {
		a.tableModel.SetResources(all)
		a.header.Update(all, len(a.factory.Clients()))
	})
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
			a.onWatchUpdate()
			return nil
		}
	}
	return event
}
