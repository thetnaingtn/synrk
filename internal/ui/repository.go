package ui

type RepositoryWithDetails struct {
	Owner          string
	Name           string
	FullName       string
	Description    string
	RepoURL        string
	DefaultBranch  string
	Parent         string
	ParentFullName string
	ParentDeleted  bool
	Private        bool
	BehindBy       int
	Error          error
}
