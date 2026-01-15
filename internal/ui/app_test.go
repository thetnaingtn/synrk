package ui

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
)

type mockSynrk struct{}

func init() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

func (m *mockSynrk) GetForks(ctx context.Context) ([]*RepositoryWithDetails, error) {
	fmt.Println("Mock: Fetching forks...")
	repos := []*RepositoryWithDetails{
		{
			Owner:          "owner1",
			Name:           "repo1",
			FullName:       "owner1/repo1",
			Description:    "Test repository 1",
			DefaultBranch:  "main",
			Parent:         "parent1",
			ParentFullName: "parent1/repo1",
			ParentDeleted:  false,
			Private:        false,
			BehindBy:       2,
		},
		{
			Owner:          "owner2",
			Name:           "repo2",
			FullName:       "owner2/repo2",
			Description:    "Test repository 2",
			DefaultBranch:  "main",
			Parent:         "parent2",
			ParentFullName: "parent2/repo2",
			ParentDeleted:  false,
			Private:        false,
			BehindBy:       2,
		},
	}

	return repos, nil
}

func (m *mockSynrk) SyncBranchWithUpstreamRepo(repo *RepositoryWithDetails) error {
	return nil
}

func TestFullOutput(t *testing.T) {
	mock := &mockSynrk{}
	m := NewAppModel(mock)

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		if !bytes.Contains(bts, []byte("2 items")) {
			return false
		}

		if !bytes.Contains(bts, []byte("These forks require synchronization")) {
			return false
		}

		if !bytes.Contains(bts, []byte("owner1/repo1 (fork from parent1/repo1)")) {
			return false
		}

		if !bytes.Contains(bts, []byte("owner2/repo2 (fork from parent2/repo2)")) {
			return false
		}

		if !bytes.Contains(bts, []byte("owner2/repo2:main is 2 commits behind parent2:main")) {
			return false
		}

		return true
	})
}

func TestInteraction(t *testing.T) {
	mock := &mockSynrk{}
	m := NewAppModel(mock)

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(300, 100))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("These forks require synchronization"))
	})

	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(" "),
	})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("● owner1/repo1 (fork from parent1/repo1)"))
	})

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(" "),
	})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("● owner1/repo1 (fork from parent1/repo1)")) && bytes.Contains(bts, []byte("● owner2/repo2 (fork from parent2/repo2)"))
	})
}
