package app

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
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
	tviewApp     *tview.Application
	cfg          config.Config
	factory      *client.ClusterFactory
	ghPoller     *client.GitHubPoller
	pageStack    *PageStack
	header       *view.Header
	footer       *view.Footer
	dashboard    *view.Dashboard
	cmdInput     *view.CmdBar
	layout       *tview.Flex
	watchers     []dao.Watcher
	cancel       context.CancelFunc
	showMineOnly bool
	cmdActive    bool
	cmdMode      string             // ":" or "/"
	filterText   string             // active / filter (regex)
	kindFilter   model.ResourceKind // empty = all kinds
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

	// GitHub poller (optional — only if org is configured)
	if a.cfg.GitHub.Org != "" {
		poller, err := client.NewGitHubPoller(a.cfg.GitHub)
		if err != nil {
			log.Printf("WARN: GitHub Actions disabled: %v", err)
		} else {
			a.ghPoller = poller
		}
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

	if a.ghPoller != nil {
		w := dao.NewWorkflowDAO(a.ghPoller)
		a.watchers = append(a.watchers, w)
		go func() {
			if err := w.Start(ctx); err != nil && ctx.Err() == nil {
				log.Printf("github watcher error: %v", err)
			}
		}()
	}
}

func (a *App) refreshLoop(ctx context.Context) {
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

	filtered := a.filterResources(all)

	a.tviewApp.QueueUpdateDraw(func() {
		a.dashboard.Refresh(filtered)
		a.header.Update(filtered, len(a.factory.Clients()), a.showMineOnly)
	})
}

func (a *App) filterResources(resources []model.Resource) []model.Resource {
	result := resources

	// Kind filter
	if a.kindFilter != "" {
		var filtered []model.Resource
		for _, r := range result {
			if r.Kind == a.kindFilter {
				filtered = append(filtered, r)
			}
		}
		result = filtered
	}

	// Mine filter: only GHA runs by current user
	if a.showMineOnly && a.ghPoller != nil {
		username := a.ghPoller.Username()
		if username != "" {
			var filtered []model.Resource
			for _, r := range result {
				if r.Kind != model.KindWorkflowRun || r.Actor == username {
					filtered = append(filtered, r)
				}
			}
			result = filtered
		}
	}

	// Text filter: regex match across key fields
	if a.filterText != "" {
		re, err := regexp.Compile("(?i)" + a.filterText)
		if err != nil {
			// Fall back to substring match if invalid regex
			lower := strings.ToLower(a.filterText)
			var filtered []model.Resource
			for _, r := range result {
				if matchSubstring(r, lower) {
					filtered = append(filtered, r)
				}
			}
			result = filtered
		} else {
			var filtered []model.Resource
			for _, r := range result {
				if matchRegex(r, re) {
					filtered = append(filtered, r)
				}
			}
			result = filtered
		}
	}

	return result
}

func matchRegex(r model.Resource, re *regexp.Regexp) bool {
	return re.MatchString(r.Name) ||
		re.MatchString(r.Cluster) ||
		re.MatchString(string(r.Kind)) ||
		re.MatchString(r.Namespace) ||
		re.MatchString(r.Message) ||
		re.MatchString(r.Repo) ||
		re.MatchString(r.Branch) ||
		re.MatchString(r.Actor) ||
		re.MatchString(r.Health.String())
}

func matchSubstring(r model.Resource, lower string) bool {
	return strings.Contains(strings.ToLower(r.Name), lower) ||
		strings.Contains(strings.ToLower(r.Cluster), lower) ||
		strings.Contains(strings.ToLower(string(r.Kind)), lower) ||
		strings.Contains(strings.ToLower(r.Namespace), lower) ||
		strings.Contains(strings.ToLower(r.Repo), lower) ||
		strings.Contains(strings.ToLower(r.Branch), lower) ||
		strings.Contains(strings.ToLower(r.Actor), lower) ||
		strings.Contains(strings.ToLower(r.Health.String()), lower)
}

func (a *App) toggleMine() {
	a.showMineOnly = !a.showMineOnly
	a.redraw()
	a.updateFooter()
}

func (a *App) onDescribe(r model.Resource) {
	dv := view.NewDescribeView(r)
	a.pageStack.Push("describe", dv)
}

func (a *App) handleInput(event *tcell.EventKey) *tcell.EventKey {
	// When cmd bar is active, only handle Ctrl+C — let the cmd bar handle everything else
	if a.cmdActive {
		if event.Key() == tcell.KeyCtrlC {
			a.Stop()
			return nil
		}
		return event
	}

	switch event.Key() {
	case tcell.KeyCtrlC:
		a.Stop()
		return nil
	case tcell.KeyEscape:
		if (a.filterText != "" || a.kindFilter != "") && a.pageStack.Current() == "dashboard" {
			a.filterText = ""
			a.kindFilter = ""
			a.redraw()
			a.updateFooter()
			return nil
		}
		if a.pageStack.Current() != "dashboard" {
			a.pageStack.Pop()
			return nil
		}
		return event
	case tcell.KeyRune:
		switch event.Rune() {
		case 'q':
			a.Stop()
			return nil
		case 'r':
			a.redraw()
			return nil
		case 'm':
			a.toggleMine()
			return nil
		case 'd':
			if a.pageStack.Current() == "dashboard" {
				a.dashboard.DescribeSelected()
				return nil
			}
			return event
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
	a.cmdActive = true
	a.cmdMode = prefix
	a.cmdInput.Activate(prefix)
	a.layout.RemoveItem(a.footer)
	a.layout.AddItem(a.cmdInput, 1, 0, true)
	a.tviewApp.SetFocus(a.cmdInput)
}

func (a *App) hideCmdBar() {
	a.cmdActive = false
	a.layout.RemoveItem(a.cmdInput)
	a.layout.AddItem(a.footer, 1, 0, false)
	a.tviewApp.SetFocus(a.pageStack)
}

func (a *App) onCmdCancel() {
	a.hideCmdBar()
}

func (a *App) onCommand(text string) {
	mode := a.cmdMode
	a.hideCmdBar()

	if mode == "/" {
		a.setFilter(text)
		return
	}

	cmd := ParseCommand(text)
	switch cmd.Name {
	case "config":
		a.execConfig()
	case "q", "quit":
		a.Stop()
	case "filter":
		if len(cmd.Args) > 0 {
			a.setFilter(strings.Join(cmd.Args, " "))
		} else {
			a.clearFilter()
		}
	case "nofilter", "nf":
		a.clearFilter()
	case "gha", "actions", "workflows":
		a.setKindFilter(model.KindWorkflowRun)
	case "ks", "kustomization", "kustomizations":
		a.setKindFilter(model.KindKustomization)
	case "hr", "helmrelease", "helmreleases":
		a.setKindFilter(model.KindHelmRelease)
	case "gitrepo", "gitrepository", "gitrepositories":
		a.setKindFilter(model.KindGitRepository)
	case "all", "dash", "dashboard":
		a.clearKindFilter()
	}
}

func (a *App) setFilter(text string) {
	a.filterText = text
	a.redraw()
	a.updateFooter()
}

func (a *App) clearFilter() {
	a.filterText = ""
	a.redraw()
	a.updateFooter()
}

func (a *App) setKindFilter(kind model.ResourceKind) {
	a.kindFilter = kind
	a.redraw()
	a.updateFooter()
}

func (a *App) clearKindFilter() {
	a.kindFilter = ""
	a.redraw()
	a.updateFooter()
}

func (a *App) updateFooter() {
	a.tviewApp.QueueUpdateDraw(func() {
		a.footer.Update(a.filterText, a.showMineOnly, string(a.kindFilter))
	})
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
	a.ghPoller = nil
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

	if a.cfg.GitHub.Org != "" {
		poller, err := client.NewGitHubPoller(a.cfg.GitHub)
		if err != nil {
			log.Printf("WARN: GitHub Actions disabled: %v", err)
		} else {
			a.ghPoller = poller
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.startWatchers(ctx)
	go a.refreshLoop(ctx)
}
