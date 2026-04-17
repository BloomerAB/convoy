package view

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const helpText = `[#6EB5FF::b]Keys[-::-]

  [#FFFFFF]a[-]          Toggle show all / active only
  [#FFFFFF]m[-]          Toggle mine / all (GitHub Actions)
  [#FFFFFF]d[-]          Describe selected resource
  [#FFFFFF]l[-]          View GitHub Actions run jobs/steps
  [#FFFFFF]o[-]          Open in browser (GitHub Actions)
  [#FFFFFF]c[-]          Copy URL to clipboard
  [#FFFFFF]r[-]          Force refresh
  [#FFFFFF]/[-]          Filter (regex across all fields)
  [#FFFFFF]:[-]          Command mode
  [#FFFFFF]Esc[-]        Clear filter / go back
  [#FFFFFF]q[-]          Quit

[#6EB5FF::b]Commands[-::-]

  [#FFFFFF]:config[-]            Edit configuration
  [#FFFFFF]:ks[-]  :kustomize    Kustomizations (all)
  [#FFFFFF]:hr[-]  :helmrelease  HelmReleases (all)
  [#FFFFFF]:helmrepo[-]          HelmRepositories (all)
  [#FFFFFF]:gitrepo[-]           GitRepositories (all)
  [#FFFFFF]:gha[-] :actions      GitHub Actions (active only)
  [#FFFFFF]:all[-] :dashboard    Back to dashboard
  [#FFFFFF]:filter[-] <text>     Set filter
  [#FFFFFF]:nofilter[-] :nf      Clear filter
  [#FFFFFF]:q[-]   :quit         Quit

[#6EB5FF::b]Dashboard[-::-]

  Default view shows only failing and syncing resources.
  Press [#FFFFFF]a[-] to toggle showing all resources.
  Press [#FFFFFF]m[-] to filter GitHub Actions to your runs.

[#9696B4]Press Esc or ? to close[-]`

func NewHelpView() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true).
		SetTitle(" Help ").
		SetBorderColor(tcell.ColorCornflowerBlue)
	tv.SetText(helpText)
	return tv
}
