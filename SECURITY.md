# Security

Vantage can edit `~/.ssh/config`, restore workspaces that may run commands, and
control tmux sessions. Please report problems in those areas privately.

## Reporting a vulnerability

Use GitHub's private reporting: open the **Security** tab of this repository and
choose **Report a vulnerability**. Please do not open a public issue.

Include what you did, what you expected, and what happened. A minimal workspace
file or ssh config (with any real hosts removed) helps.

## Scope

In scope: unintended command execution, writes outside the intended files,
corruption of `~/.ssh/config` or workspace files, and anything that reads or
exposes ssh keys or credentials.

This is a personal project maintained on a best-effort basis, so there is no
fixed response time.
