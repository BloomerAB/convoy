package app

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
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

func drawHLine(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
	style := tcell.StyleDefault.Foreground(tcell.ColorDimGray).Background(tcell.ColorDefault)
	for i := x; i < x+width; i++ {
		screen.SetContent(i, y, '─', nil, style)
	}
	return x, y, width, height
}

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
	showAll      bool               // false = only failed/progressing (default)
	cmdActive    bool
	cmdMode      string             // ":" or "/"
	filterText   string             // active / filter (regex)
	kindFilter     model.ResourceKind  // empty = all kinds
	runLogResource *model.Resource     // resource shown in current runlog view
	kindView       *view.KindView     // active kind page (nil when on dashboard)
	treeView       *view.TreeView     // active tree view (nil when not on tree)
	ghaTreeView    *view.GHATreeView  // active GHA tree view

	// snapshot is the latest resource collection, updated by background goroutine.
	// The UI goroutine only reads this — never touches watcher locks.
	snapshot atomic.Value // []model.Resource
}

func New(cfg config.Config) *App {
	a := &App{
		tviewApp: tview.NewApplication(),
		cfg:      cfg,
		factory:  client.NewClusterFactory(),
	}
	a.snapshot.Store([]model.Resource(nil))
	return a
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

	headerSep := tview.NewBox().SetBorder(false).SetDrawFunc(drawHLine)
	footerSep := tview.NewBox().SetBorder(false).SetDrawFunc(drawHLine)

	a.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(headerSep, 1, 0, false).
		AddItem(a.pageStack, 0, 1, true).
		AddItem(footerSep, 1, 0, false).
		AddItem(a.footer, 1, 0, false)

	a.tviewApp.SetRoot(a.layout, true)
	a.tviewApp.SetInputCapture(a.handleInput)

	return nil
}

func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.startWatchers(ctx)
	go a.collectorLoop(ctx)
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
			dao.NewHelmRepositoryDAO(cc, func() {}),
			dao.NewGitRepositoryDAO(cc, func() {}),
			dao.NewDeploymentDAO(cc),
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

// collectorLoop runs on a background goroutine, collecting from watchers
// (which may block on locks) and storing a snapshot. Never runs on UI goroutine.
func (a *App) collectorLoop(ctx context.Context) {
	// Initial collection after brief delay
	time.Sleep(500 * time.Millisecond)
	a.updateSnapshot()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.updateSnapshot()
		}
	}
}

func (a *App) updateSnapshot() {
	var all []model.Resource
	for _, w := range a.watchers {
		all = append(all, w.Resources()...)
	}
	a.snapshot.Store(all)
}

func (a *App) getSnapshot() []model.Resource {
	v := a.snapshot.Load()
	if v == nil {
		return nil
	}
	return v.([]model.Resource)
}

// refreshLoop redraws the UI on a fixed interval using the snapshot.
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

// redraw queues a UI update from a background goroutine.
func (a *App) redraw() {
	all := a.getSnapshot()
	filtered := a.filterResources(all)

	a.tviewApp.QueueUpdateDraw(func() {
		a.applyUpdate(all, filtered)
	})
}

// redrawDirect updates the UI immediately — call from the UI goroutine only.
func (a *App) redrawDirect() {
	all := a.getSnapshot()
	filtered := a.filterResources(all)
	a.applyUpdate(all, filtered)
}

func (a *App) applyUpdate(all []model.Resource, filtered []model.Resource) {
	a.dashboard.Refresh(filtered, a.showAll)
	if a.kindView != nil {
		a.kindView.Refresh(all)
	}
	if a.treeView != nil {
		a.treeView.Refresh(all)
	}
	if a.ghaTreeView != nil {
		a.ghaTreeView.Refresh(all)
	}
	a.header.Update(all, len(a.factory.Clients()), a.showMineOnly, a.showAll)
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
	a.redrawDirect()
	a.updateFooterDirect()
}

func (a *App) toggleShowAll() {
	a.showAll = !a.showAll
	a.redrawDirect()
	a.updateFooterDirect()
}

// selectedResource returns the resource under the cursor in whatever list view is active.
func (a *App) selectedResource() *model.Resource {
	cur := a.pageStack.Current()
	switch {
	case cur == "dashboard":
		return a.dashboard.SelectedResource()
	case a.kindView != nil && strings.HasPrefix(cur, "kind-"):
		return a.kindView.SelectedResource()
	case cur == "runlog":
		return a.runLogResource
	case cur == "tree" && a.treeView != nil:
		return a.treeView.SelectedResource()
	case cur == "github" && a.ghaTreeView != nil:
		return a.ghaTreeView.SelectedResource()
	}
	return nil
}

func (a *App) reconcileOrRerun() {
	r := a.selectedResource()
	if r == nil {
		return
	}

	switch r.Kind {
	case model.KindWorkflowRun:
		if a.ghPoller == nil {
			return
		}
		a.header.Flash("⟳ rerunning " + r.Name)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := a.ghPoller.RerunWorkflow(ctx, r.Repo, r.RunID, r.Health.IsFailed())
			if err != nil {
				log.Printf("rerun workflow: %v", err)
				a.header.Flash("✗ rerun failed: " + err.Error())
			} else {
				log.Printf("rerun triggered: %s/%s", r.Repo, r.Name)
				a.header.Flash("✓ rerun triggered: " + r.Name)
			}
		}()
	case model.KindKustomization, model.KindHelmRelease, model.KindGitRepository, model.KindHelmRepository:
		a.header.Flash("⟳ reconciling " + r.Name)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := a.factory.Reconcile(ctx, *r)
			if err != nil {
				log.Printf("reconcile: %v", err)
				a.header.Flash("✗ reconcile failed: " + err.Error())
			} else {
				log.Printf("reconcile triggered: %s/%s on %s", r.Namespace, r.Name, r.Cluster)
				a.header.Flash("✓ reconciled: " + r.Name)
			}
		}()
	}
}

func (a *App) suspendResource() {
	r := a.selectedResource()
	if r == nil || r.Kind == model.KindWorkflowRun {
		return
	}
	a.header.Flash("◌ suspending " + r.Name)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := a.factory.Suspend(ctx, *r)
		if err != nil {
			log.Printf("suspend: %v", err)
			a.header.Flash("✗ suspend failed: " + err.Error())
		} else {
			log.Printf("suspended: %s/%s on %s", r.Namespace, r.Name, r.Cluster)
			a.header.Flash("◌ suspended: " + r.Name)
		}
	}()
}

func (a *App) resumeResource() {
	r := a.selectedResource()
	if r == nil || r.Kind == model.KindWorkflowRun {
		return
	}
	a.header.Flash("▶ resuming " + r.Name)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := a.factory.Resume(ctx, *r)
		if err != nil {
			log.Printf("resume: %v", err)
			a.header.Flash("✗ resume failed: " + err.Error())
		} else {
			log.Printf("resumed: %s/%s on %s", r.Namespace, r.Name, r.Cluster)
			a.header.Flash("✓ resumed: " + r.Name)
		}
	}()
}

func (a *App) openInBrowser() {
	if r := a.selectedResource(); r != nil && r.URL != "" {
		_ = exec.Command("open", r.URL).Start()
	}
}

func (a *App) copyResource() {
	r := a.selectedResource()
	if r == nil {
		return
	}

	// URL if available, otherwise resource summary
	var text string
	if r.URL != "" {
		text = r.URL
	} else {
		text = fmt.Sprintf("%s %s/%s (%s) on %s: %s",
			r.Health.String(), r.Namespace, r.Name, r.Kind, r.Cluster, r.Message)
	}

	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
	a.header.Flash("copied: " + r.Name)
}

func (a *App) showRunLogFor(r *model.Resource) {
	if a.ghPoller == nil {
		return
	}

	a.runLogResource = r
	lv := view.NewRunLogView(*r)
	a.pageStack.Push("runlog", lv)

	// Fetch jobs in background, update view when done
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		content, err := a.ghPoller.FetchRunJobs(ctx, r.Repo, r.RunID)
		a.tviewApp.QueueUpdateDraw(func() {
			if err != nil {
				lv.SetError(err)
			} else {
				lv.SetContent(content)
			}
		})
	}()
}

func (a *App) showTree() {
	a.showClusterPicker()
}

func (a *App) showTreeForCluster(cluster string) {
	all := a.getSnapshot()
	tv := view.NewFluxTreeView(all, cluster)
	a.treeView = tv
	a.pageStack.Push("tree", tv)
}

func (a *App) showClusterPicker() {
	all := a.getSnapshot()
	clusters := make(map[string]bool)
	for _, r := range all {
		if r.Kind != model.KindWorkflowRun {
			clusters[r.Cluster] = true
		}
	}

	var names []string
	for c := range clusters {
		names = append(names, c)
	}
	sort.Strings(names)

	list := tview.NewList()
	for _, name := range names {
		n := name
		list.AddItem(n, "", 0, func() {
			a.showTreeForCluster(n)
		})
	}
	list.SetBorderPadding(0, 0, 1, 1)

	a.pageStack.Push("tree-picker", list)
}

func (a *App) showGHATree() {
	all := a.getSnapshot()
	tv := view.NewGHATreeView(all)
	a.ghaTreeView = tv
	a.pageStack.Push("github", tv)
}

func (a *App) showHelp() {
	hv := view.NewHelpView()
	a.pageStack.Push("help", hv)
}

func (a *App) onDescribe(r model.Resource) {
	dv := view.NewDescribeView(r)
	a.pageStack.Push("describe", dv)
}

func (a *App) handleInput(event *tcell.EventKey) *tcell.EventKey {
	// When cmd bar is active, only handle Ctrl+C
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
			a.redrawDirect()
			a.updateFooterDirect()
			return nil
		}
		if a.pageStack.Current() != "dashboard" {
			if strings.HasPrefix(a.pageStack.Current(), "kind-") {
				a.kindView = nil
			}
			if a.pageStack.Current() == "tree" {
				a.treeView = nil
			}
			if a.pageStack.Current() == "github" {
				a.ghaTreeView = nil
			}
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
			a.reconcileOrRerun()
			return nil
		case 'R':
			a.redrawDirect()
			return nil
		case 'm':
			a.toggleMine()
			return nil
		case 'a':
			a.toggleShowAll()
			return nil
		case 's':
			a.suspendResource()
			return nil
		case 'u':
			a.resumeResource()
			return nil
		case 'd':
			if r := a.selectedResource(); r != nil {
				a.onDescribe(*r)
				return nil
			}
			return event
		case 'l':
			if r := a.selectedResource(); r != nil && r.Kind == model.KindWorkflowRun {
				a.showRunLogFor(r)
				return nil
			}
			return event
		case 'o':
			a.openInBrowser()
			return nil
		case 'c':
			a.copyResource()
			return nil
		case 't':
			a.showTree()
			return nil
		case '?':
			a.showHelp()
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
	case "gha", "actions", "workflows", "github":
		a.showGHATree()
	case "ks", "kustomize", "kustomization", "kustomizations":
		a.pushKindView(model.KindKustomization, false)
	case "hr", "helmrelease", "helmreleases":
		a.pushKindView(model.KindHelmRelease, false)
	case "helmrepo", "helmrepository", "helmrepositories":
		a.pushKindView(model.KindHelmRepository, false)
	case "gitrepo", "gitrepository", "gitrepositories":
		a.pushKindView(model.KindGitRepository, false)
	case "all", "dash", "dashboard":
		a.popToHome()
	case "tree":
		if len(cmd.Args) > 0 {
			a.showTreeForCluster(cmd.Args[0])
		} else {
			a.showTree()
		}
	}
}

func (a *App) setFilter(text string) {
	a.filterText = text
	a.redrawDirect()
	a.updateFooterDirect()
}

func (a *App) clearFilter() {
	a.filterText = ""
	a.redrawDirect()
	a.updateFooterDirect()
}

func (a *App) pushKindView(kind model.ResourceKind, activeOnly bool) {
	kv := view.NewKindView(kind, activeOnly, a.onDescribe)
	a.kindView = kv

	all := a.getSnapshot()
	kv.Refresh(all)

	a.pageStack.Push("kind-"+string(kind), kv)
}

func (a *App) popToHome() {
	a.pageStack.PopTo("dashboard")
	a.kindView = nil
	a.treeView = nil
	a.ghaTreeView = nil
}

func (a *App) updateFooterDirect() {
	a.footer.Update(a.filterText, a.showMineOnly, a.showAll, string(a.kindFilter))
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
	go a.collectorLoop(ctx)
	go a.refreshLoop(ctx)
}
