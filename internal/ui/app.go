package ui

import (
	"context"
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type repository interface {
	GetForks(ctx context.Context) ([]*RepositoryWithDetails, error)
	SyncBranchWithUpstreamRepo(repo *RepositoryWithDetails) error
}

type AppModel struct {
	synrk repository
	err   error
	list  list.Model
}

func (m AppModel) toggleSelection() tea.Cmd {
	idx := m.list.Index()
	item := m.list.SelectedItem().(item)
	if item.repo.Error != nil {
		return nil
	}
	item.selected = !item.selected
	m.list.RemoveItem(idx)

	return m.list.InsertItem(idx, item)
}

func (m AppModel) changeSelect(selected bool) []tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.list.Items()))

	for idx, i := range m.list.Items() {
		item := i.(item)
		if item.repo.Error != nil {
			continue
		}
		item.selected = selected
		m.list.RemoveItem(idx)
		cmds = append(cmds, m.list.InsertItem(idx, item))
	}

	return cmds
}

func (m AppModel) selectAtleastOne() bool {
	for _, i := range m.list.Items() {
		item := i.(item)

		if item.selected {
			return true
		}
	}

	return false
}

func (m AppModel) isAllSelectedReposSynced() bool {
	for _, i := range m.list.Items() {
		item := i.(item)
		if item.selected && !item.synced {
			return false
		}
	}

	return true
}

func NewAppModel(synrk repository) AppModel {
	list := newList()
	return AppModel{synrk: synrk, list: list}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(enqueuegetReposListCmd, m.list.StartSpinner())
}

func (m AppModel) View() string {
	if m.err != nil {
		return errorStyle.Bold(true).Render("Can't get the forks at the moment 😭") + "\n" + m.err.Error()
	}
	return m.list.View()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	var cmds []tea.Cmd
	var cmd tea.Cmd

	ctx := context.Background()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Println("tea.WindowSizeMsg")
		top, right, bottom, left := listStyle.GetMargin()
		m.list.SetSize(msg.Width-left-right, msg.Height-top-bottom)
	case refreshReposListMsg:
		log.Println("refreshReposListCmd")
		m.list.Title = "🍀 Refreshing forks"
		cmds = append(cmds, m.list.StartSpinner(), m.getReposCmd(ctx))
	case getReposListMsg:
		log.Println("getReposListCmd")
		m.list.Title = "Getting forks. Hold tight!"
		m.list.SetShowStatusBar(false)
		m.list.SetShowHelp(false)
		cmds = append(cmds, m.list.StartSpinner(), m.getReposCmd(ctx))
	case gotReposListMsg:
		log.Println("gotReposListCmd")
		m.list.Title = "All forks are up to date 🤗"
		if len(msg.repos) > 0 {
			m.list.Title = fmt.Sprintf("🤔 These fork%s require synchronization", mayBePlural(len(msg.repos)))
		}
		m.list.StopSpinner()
		m.list.SetShowStatusBar(true)
		m.list.SetShowHelp(true)
		cmds = append(cmds, m.list.SetItems(reposToItems(msg.repos)))
	case mergeSelectedReposMsg:
		m.list.Title = "Syncing with upstream repository!"
		items := m.list.Items()
		cmds = append(cmds, m.mergeReposCmd(items))
	case mergedSelectedReposMsg:
		m.list.Title = "✅ Synchronization Done"
		m.list.StopSpinner()
		cmds = append(cmds, m.list.SetItems(msg.items))
	case errorMsg:
		m.err = msg.error
	// key messages
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

		if key.Matches(msg, keySelectAll) {
			cmds = append(cmds, m.changeSelect(true)...)
		}

		if key.Matches(msg, keySelectNone) {
			cmds = append(cmds, m.changeSelect(false)...)
		}

		if key.Matches(msg, keySelectToggle) {
			cmds = append(cmds, m.toggleSelection())
		}

		if key.Matches(msg, keyMergeWithUpstream) {
			if !m.selectAtleastOne() {
				cmds = append(cmds, m.list.NewStatusMessage(listStatusStyle.Render("💡 No repo selected")))
			} else if m.isAllSelectedReposSynced() {
				cmds = append(cmds, m.list.NewStatusMessage(listStatusStyle.Render("💡 All selected repositories are synced")))
			} else {
				cmds = append(cmds, m.list.StartSpinner(), requestMergeReposCmd)
			}
		}

		if key.Matches(msg, keyRefresh) {
			cmds = append(cmds, m.list.StartSpinner(), refreshReposListCmd)
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
