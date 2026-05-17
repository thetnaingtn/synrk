# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`synrk` is a Go TUI (Bubble Tea) CLI that lists your GitHub forks, computes how many commits each is behind its upstream's default branch, and lets you select forks and sync them via GitHub's "merge upstream" API.

## Common commands

- Build: `go build ./cmd/synrk`
- Run locally: `go run ./cmd/synrk --token <gh-token>` (or set `GITHUB_TOKEN`); add `-f` to include forks updated within the last 24h.
- All tests: `go test ./... -v` (this is what CI runs in `.github/workflows/test.yml`).
- Single test: `go test ./internal/ui -run TestName -v`.
- Runtime log file (Bubble Tea): `$TMPDIR/synrk.log` — tail this when debugging the UI, since stdout is taken by the alt-screen.

Go module is `github.com/thetnaingtn/synrk`, Go 1.23.

## Architecture

Two layers, deliberately separated so the GitHub logic can be tested without the TUI:

- `internal/synrk` — the `Synrk` interface (`GetForks`, `SyncBranchWithUpstreamRepo`) and its `concrete` implementation wrapping `google/go-github/v52`.
  - `GetForks` pages through `Repositories.List` (type=owner), filters to forks, optionally drops forks updated in the last 24h (`force=false`), then fans out one goroutine per remaining fork to call `Repositories.Get` + `Repositories.CompareCommits` (base = `parent_owner:default_branch`, head = `fork_owner:default_branch`). Results stream back over a buffered channel and are sorted by `BehindBy` desc.
  - A 404 on the compare is treated as "parent branch deleted" — the row is still returned with `ParentDeleted=true` and an `Error` so the UI can show it as non-actionable.
  - `SyncBranchWithUpstreamRepo` calls `Repositories.MergeUpstream`; 409 is surfaced as a conflict error.

- `internal/ui` — Bubble Tea `AppModel` built around `bubbles/list`. `app.go` is the `Update` loop; `commands.go` defines the `tea.Cmd`s and message types (`getReposListMsg`, `refreshReposListMsg`, `mergeSelectedReposMsg`, `mergedSelectedReposMsg`, `errorMsg`); `item.go` wraps `RepositoryWithDetails` with `selected`/`synced` flags; `key.go` defines keybindings (a/n/space/r/m/q); `styles.go` holds lipgloss styles.
  - Selection toggles work by removing and re-inserting the list item (bubbles `list.Model` has no in-place mutation), so any change to selection state goes through `RemoveItem` + `InsertItem`.
  - `AppModel` skips items with `repo.Error != nil` when bulk-selecting or merging.

- `cmd/synrk/main.go` wires `urfave/cli/v2` flags (`--token`/`-t`, `--force`/`-f`), reads the token from `GITHUB_TOKEN` first then the flag, builds an OAuth-authed `github.Client`, and starts Bubble Tea with `tea.WithAltScreen()`.

## Release / distribution

- Tagging `v*.*.*` triggers `.github/workflows/release.yml`: GoReleaser builds binaries (config in `.goreleaser.yaml`), then a matrix job packages per-OS/arch npm packages from `npm/package.json.tmpl` (`synrk-<os>-<arch>`) and publishes them, and finally `npm/synrk` (the meta package with `optionalDependencies` for each platform binary) is built with pnpm/tsc and published. Bumping the version means updating `npm/synrk/package.json` `version` and each entry in `optionalDependencies` to match the new tag — see commit `a9f2c36` for the pattern.
- Changelog config: `cliff.toml` (git-cliff).
