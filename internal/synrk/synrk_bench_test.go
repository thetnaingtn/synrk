package synrk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v52/github"
)

func setupMockServer() *httptest.Server {
	mux := http.NewServeMux()

	// Mock repository get endpoint
	mux.HandleFunc("/repos/testowner/test-repo", func(w http.ResponseWriter, r *http.Request) {
		repo := &github.Repository{
			Name:          github.String("test-repo"),
			FullName:      github.String("testowner/test-repo"),
			DefaultBranch: github.String("main"),
			Private:       github.Bool(false),
			Owner: &github.User{
				Login: github.String("testowner"),
			},
			Parent: &github.Repository{
				Name:     github.String("parent-repo"),
				FullName: github.String("parentowner/parent-repo"),
				Owner: &github.User{
					Login: github.String("parentowner"),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repo)
	})

	// Mock compare commits endpoint
	mux.HandleFunc("/repos/testowner/test-repo/compare/parentowner:main...testowner:main", func(w http.ResponseWriter, r *http.Request) {
		comparison := &github.CommitsComparison{
			BehindBy: github.Int(5),
			AheadBy:  github.Int(0),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comparison)
	})

	return httptest.NewServer(mux)
}

func createMockForks(n int) []*github.Repository {
	forks := make([]*github.Repository, n)
	oldTime := time.Now().Add(-48 * time.Hour) // 2 days ago, so it passes the recently updated check

	for i := 0; i < n; i++ {
		forks[i] = &github.Repository{
			Name:     github.String("test-repo"),
			FullName: github.String("testowner/test-repo"),
			Fork:     github.Bool(true),
			Owner: &github.User{
				Login: github.String("testowner"),
			},
			UpdatedAt: &github.Timestamp{Time: oldTime},
		}
	}
	return forks
}

func BenchmarkGetReposDetail(b *testing.B) {
	benchmarks := []struct {
		name     string
		numForks int
	}{
		{"1_fork", 1},
		{"5_forks", 5},
		{"10_forks", 10},
		{"25_forks", 25},
		{"50_forks", 50},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			server := setupMockServer()
			defer server.Close()

			client := github.NewClient(nil)
			client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

			synrkClient := &concrete{
				client:   client,
				pageSize: 100,
				force:    true,
			}

			forks := createMockForks(bm.numForks)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				synrkClient.getReposDetail(ctx, forks)
			}
		})
	}
}

