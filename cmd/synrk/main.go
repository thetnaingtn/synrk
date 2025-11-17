package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-github/v52/github"
	"github.com/thetnaingtn/synrk/internal/synrk"
	"github.com/thetnaingtn/synrk/internal/ui"
	"github.com/urfave/cli/v2"
)

func getGitHubToken(c *cli.Context) string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	// Fallback to CLI flag
	return c.String("token")
}

func main() {
	app := cli.NewApp()

	app.Authors = []*cli.Author{{
		Name:  "thetnaingtn",
		Email: "thetnaingtun.ucsy@gmail.com",
	}}
	app.Usage = "Synchronize your forks with ease."

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			EnvVars: []string{"GITHUB_TOKEN"},
			Name:    "token",
			Usage:   "Your Github Token.",
			Aliases: []string{"t"},
		},
		&cli.BoolFlag{
			Name:    "force",
			Usage:   "Force to fetch all forks even some of them are recently updated.",
			Value:   false,
			Aliases: []string{"f"},
		},
	}

	app.Action = func(c *cli.Context) error {

		log.SetFlags(0)
		f, err := tea.LogToFile(filepath.Join(os.TempDir(), "synrk.log"), "")
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		defer func() { _ = f.Close() }()

		token := getGitHubToken(c)

		if token == "" {
			return cli.Exit("missing github token", 1)
		}

		force := c.Bool("force")

		ctx := context.Background()

		client := github.NewTokenClient(ctx, token)

		_synrk := synrk.NewSynrk(client, 100, force)

		m := ui.NewAppModel(_synrk)

		p := tea.NewProgram(m, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			return cli.Exit(err.Error(), 1)
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalln(err)
	}

}
