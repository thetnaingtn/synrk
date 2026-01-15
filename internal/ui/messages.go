package ui

import (
	"github.com/charmbracelet/bubbles/list"
)

type errorMsg struct {
	error
}

func (e errorMsg) Error() string {
	return e.error.Error()
}

type refreshReposListMsg struct{}

type getReposListMsg struct{}
type gotReposListMsg struct {
	repos []*RepositoryWithDetails
}

type mergeSelectedReposMsg struct{}
type mergedSelectedReposMsg struct {
	items []list.Item
}
