# Contributing

1. Read `AGENTS.md` (the hard rules), `go-conventions.md` (how we write Go here), and skim `architecture.md` (what is decided and what is planned).
2. Setup: install Go (version in `go.mod`), tmux, and [golangci-lint v2](https://golangci-lint.run/welcome/install/), then `make check`.
3. Pick work from the roadmap in `architecture.md`, or open an issue first for anything that changes a decision there.
4. Put code where `layout.md` says. Add tests next to it. Tests that use tmux must use an isolated socket (`workflow.md`).
5. Changing the TOML schema? Update `workspace-format.md` and `examples/` in the same change.
6. Run `make check`, then open a PR with a Conventional Commit title.

## Adding a feature: checklist

- [ ] Code in the right package, dependencies pointing inward
- [ ] Tests, including bad input for anything that reaches a shell or tmux target
- [ ] Works on macOS and Linux/WSL (no hardcoded paths)
- [ ] Docs updated (schema, architecture, or roadmap status)
- [ ] `make check` passes
