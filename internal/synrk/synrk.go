package synrk

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/google/go-github/v52/github"
)

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

type Synrk interface {
	GetForks(ctx context.Context) ([]*RepositoryWithDetails, error)
	SyncBranchWithUpstreamRepo(repo *RepositoryWithDetails) error
}

type concrete struct {
	client   *github.Client
	pageSize int
	force    bool
}

func NewSynrk(client *github.Client, pageSize int, force bool) Synrk {
	return &concrete{
		client:   client,
		pageSize: pageSize,
		force:    force,
	}
}

func (c *concrete) GetForks(ctx context.Context) ([]*RepositoryWithDetails, error) {
	forks, err := c.getAllForks(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch fork list: %w\n", err)
	}

	forksWithDetails := c.getReposDetail(ctx, forks)

	sort.SliceStable(forksWithDetails, func(i, j int) bool {
		return forksWithDetails[i].BehindBy > forksWithDetails[j].BehindBy
	})

	return forksWithDetails, nil
}

func (c *concrete) SyncBranchWithUpstreamRepo(repo *RepositoryWithDetails) error {
	request := &github.RepoMergeUpstreamRequest{Branch: &repo.DefaultBranch}
	res, resp, err := c.client.Repositories.MergeUpstream(context.Background(), repo.Owner, repo.Name, request)

	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("couldn't merge with upstream %s branch due to conflict", res.GetBaseBranch())
	}

	if err != nil {
		return fmt.Errorf("couldn't merge with upstream %s branch: %w", res.GetBaseBranch(), err)
	}

	return nil
}

func (c *concrete) getReposDetail(ctx context.Context, forks []*github.Repository) []*RepositoryWithDetails {
	done := make(chan any)

	defer close(done)

	forksRequiredSync := []*github.Repository{}

	for _, fork := range forks {
		ghTimeStamp := fork.GetUpdatedAt()
		if !c.force && c.doesForkRecentlyUpdated(ghTimeStamp.GetTime()) {
			continue
		}
		forksRequiredSync = append(forksRequiredSync, fork)
	}

	forkStream := make(chan *RepositoryWithDetails, len(forksRequiredSync))
	defer close(forkStream)

	for _, fork := range forksRequiredSync {
		go func() {
			select {
			case <-done:
				return
			default:
				repo, _, err := c.client.Repositories.Get(ctx, fork.GetOwner().GetLogin(), fork.GetName())
				if err != nil {
					log.Println("getReposDetail", err)
					forkStream <- &RepositoryWithDetails{Error: fmt.Errorf("failed to get repository %s: %w", fork.GetName(), err)}
					return
				}

				parent := repo.GetParent()

				base := fmt.Sprintf("%s:%s", parent.GetOwner().GetLogin(), repo.GetDefaultBranch()) // compare with forked repo's default branch
				head := fmt.Sprintf("%s:%s", repo.GetOwner().GetLogin(), repo.GetDefaultBranch())

				cmpr, resp, err := c.client.Repositories.CompareCommits(
					ctx,
					repo.GetOwner().GetLogin(),
					repo.GetName(),
					base,
					head,
					&github.ListOptions{},
				)

				if err != nil && resp.StatusCode == http.StatusNotFound {
					log.Println("getReposDetail", err)
					repoWithDetail := c.buildDetails(repo, nil, resp.StatusCode)
					repoWithDetail.Error = fmt.Errorf("can't find %s branch on %s", head, parent.GetFullName())
					forkStream <- repoWithDetail
					return
				}

				if err != nil && resp.StatusCode != http.StatusNotFound {
					log.Println("getReposDetail", err)
					repoWithDetail := c.buildDetails(repo, nil, resp.StatusCode)
					repoWithDetail.Error = fmt.Errorf("failed to compare repository with parent %s: %w", parent.GetName(), err)
					forkStream <- repoWithDetail
					return
				}

				forkStream <- c.buildDetails(repo, cmpr, resp.StatusCode)
			}

		}()
	}

	forksWithDetails := make([]*RepositoryWithDetails, 0, len(forksRequiredSync))
	for range len(forksRequiredSync) {
		fork := <-forkStream
		forksWithDetails = append(forksWithDetails, fork)
	}

	return forksWithDetails
}

func (c *concrete) buildDetails(repo *github.Repository, commit *github.CommitsComparison, code int) *RepositoryWithDetails {
	repoWithDetails := &RepositoryWithDetails{
		ParentDeleted: code == http.StatusNotFound,
	}

	if commit != nil {
		repoWithDetails.BehindBy = commit.GetBehindBy()
	}

	if repo != nil {
		repoWithDetails.Owner = repo.GetOwner().GetLogin()
		repoWithDetails.Name = repo.GetName()
		repoWithDetails.FullName = repo.GetFullName()
		repoWithDetails.Description = repo.GetDescription()
		repoWithDetails.RepoURL = repo.GetURL()
		repoWithDetails.DefaultBranch = repo.GetDefaultBranch()
		repoWithDetails.Parent = repo.GetParent().GetOwner().GetLogin()
		repoWithDetails.ParentFullName = repo.GetParent().GetFullName()
		repoWithDetails.Private = repo.GetPrivate()
	}

	return repoWithDetails
}

func (c *concrete) getAllForks(ctx context.Context) ([]*github.Repository, error) {
	var allRepos []*github.Repository
	opts := &github.RepositoryListOptions{
		Type:        "owner",
		ListOptions: github.ListOptions{PerPage: c.pageSize},
	}

	for {
		forks, resp, err := c.client.Repositories.List(ctx, "", opts)
		if err != nil {
			return allRepos, err
		}

		allRepos = append(allRepos, forks...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	forks := make([]*github.Repository, 0, len(allRepos))
	for _, repo := range allRepos {
		if !repo.GetFork() {
			continue
		}

		forks = append(forks, repo)
	}

	return forks, nil
}

func (c *concrete) doesForkRecentlyUpdated(updatedAt *time.Time) bool {
	if updatedAt == nil {
		return false
	}

	localUpdatedAt := updatedAt.In(time.Local)
	now := time.Now().In(time.Local)

	return localUpdatedAt.After(now.Add(-24 * time.Hour))
}
