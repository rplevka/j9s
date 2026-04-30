# j9s — Jenkins TUI Manager

A terminal UI for managing Jenkins instances, inspired by [k9s](https://github.com/derailed/k9s).

> **Status**: under active development. Expect breaking changes to keys/commands until 1.0.

## Features

- **k9s-style interface** — same TUI patterns, vim-like keybindings, breadcrumbs, command prompt.
- **Multi-context** — switch between Jenkins instances on the fly via `:ctx <name>`.
- **Resource views** — Jobs (with nested folders), Builds, Queue, Nodes, Users, Credentials, Plugins, Views.
- **Path-aware command prompt** — argument-aware autocomplete suggesting values from the current view, qualified with the active folder path. Typing `:builds d` from `team-a/sub` completes to `:builds team-a/sub/deploy`.
- **Live logs** — streaming console output with filtering, wrap toggle, autoscroll, mark, copy, save, full-screen, head/tail.
- **Build actions** — trigger parameterised builds (params dialog with pre-filled defaults and Build button focused), rebuild last, stop running builds, view artifacts, describe config.
- **Test reports** — JUnit-plugin browser (suites → cases → detail) with status colorization and full filter/sort; HTML Publisher launcher (pytest-html, allure, …) opens reports in the system browser.
- **Pipeline graph** — Blue Ocean style ASCII view of a Pipeline run's DAG, with parallel branches drawn using `├─/└─` markers and per-node drill-in to step lists and step/aggregated logs.
- **Job toggle** — single `Shift+E` to toggle Enable/Disable; `e` reserved for future edit.
- **Sorting** — `Shift+N`/`Shift+S`/`Shift+R`/`Shift+A` per view (name/status/result/age depending on context).
- **Bookmarks** — save the current view path and jump back later.
- **HTTP cache** — opt-in response cache with `:cache stats` / `:cache clear`.
- **Auth** — API token (recommended), password, OAuth/SSO.

## Installation

```bash
go install github.com/roman-plevka/j9s@latest
```

Or build from source (always produces `bin/j9s`):

```bash
git clone https://github.com/roman-plevka/j9s.git
cd j9s
make build      # → bin/j9s
make install    # → $GOPATH/bin/j9s
```

`make build` injects the short commit SHA into the version string, so the header shows `dev-<sha>` on local builds.

## Configuration

Config lives at `~/.config/j9s/config.yaml` (or `$XDG_CONFIG_HOME/j9s/config.yaml`).

```yaml
j9s:
  refreshRate: 2
  currentContext: my-jenkins
  contexts:
    - name: my-jenkins
      url: https://jenkins.example.com
      auth:
        type: token
        username: your-username
        token: your-api-token
    - name: jenkins-prod
      url: https://jenkins-prod.example.com
      auth:
        type: token
        username: admin
        token: admin-api-token
      insecure: true   # skip TLS verification
```

### Authentication types

**API token (recommended):**
```yaml
auth:
  type: token
  username: your-username
  token: your-api-token
```

**Password:**
```yaml
auth:
  type: password
  username: your-username
  password: your-password
```

**OAuth / SSO:**
```yaml
auth:
  type: oauth
  oauth:
    clientId: your-client-id
    clientSecret: your-client-secret
    tokenUrl: https://auth.example.com/oauth/token
    authUrl: https://auth.example.com/oauth/authorize
```

## Usage

```bash
# default context
j9s

# specific context
j9s --context my-jenkins

# launch directly into a view (resource alias from the table below)
j9s -c nodes

# read-only mode — disables destructive actions
j9s --readonly

# tweak refresh rate (seconds)
j9s -r 5

# headless / logoless for screencasts
j9s --headless --logoless

# custom log file / level
j9s --logFile /tmp/j9s.log --logLevel debug
```

## Keybindings

### Global
| Key | Action |
|-----|--------|
| `:` | Command mode |
| `/` | Filter mode |
| `?` | Help |
| `Esc` | Back / clear filter |
| `Ctrl+C` | Quit |

### Navigation (table views)
| Key | Action |
|-----|--------|
| `Enter` | Drill down (folder → contents, job → builds, build → logs) |
| `j`/`k` or `↑`/`↓` | Move row |
| `g` / `G` | Top / bottom |
| `r` | Refresh |

### Jobs view
| Key | Action |
|-----|--------|
| `Enter` | Open builds (or descend into folder) |
| `b` | Trigger build (parameter dialog if applicable) |
| `d` | Describe (job config) |
| `a` | Artifacts of last successful build |
| `l` | Logs of last build |
| `t` | JUnit test report (suites → cases → detail) |
| `h` | HTML Publisher reports (open in system browser) |
| `p` | Pipeline graph (Blue Ocean DAG) for last build |
| `v` | Switch to Views |
| `Shift+E` | Toggle Enable/Disable |
| `Ctrl+D` | Delete |
| `Shift+N` / `Shift+S` / `Shift+A` | Sort by name / status / age |

### Views (Jenkins views) and ViewJobs
| Key | Action |
|-----|--------|
| `Enter` | Open jobs in this view |
| `b` | Trigger build (ViewJobs) |
| `d` | Describe |
| `a` / `l` | Artifacts / Logs (ViewJobs) |
| `t` / `h` | JUnit tests / HTML reports (ViewJobs) |
| `p` | Pipeline graph (ViewJobs) |
| `v` | Switch to Views |

### Builds view
| Key | Action |
|-----|--------|
| `Enter` or `l` | Open logs |
| `b` | Rebuild (re-trigger with the same parameters) |
| `d` | Describe build |
| `a` | Artifacts |
| `t` | JUnit test report for this build |
| `h` | HTML Publisher reports for this build |
| `p` | Pipeline graph for this build |
| `s` | Stop running build |
| `Shift+N` / `Shift+R` / `Shift+A` | Sort by number / result / age |

### Logs view
| Key | Action |
|-----|--------|
| `/` | Filter |
| `Shift+C` | Clear filter |
| `g` / `Shift+G` | Top / bottom |
| `0` / `1` | Tail / head |
| `w` | Toggle wrap |
| `s` | Toggle autoscroll |
| `f` | Full-screen toggle |
| `m` | Mark line |
| `c` | Copy to clipboard |
| `Ctrl+S` | Save to file |
| `q` / `Esc` | Back |

### Queue
| Key | Action |
|-----|--------|
| `Ctrl+D` | Cancel queued item |

## Commands

Type `:` to enter command mode. Resource aliases:

| Command | Aliases | Description |
|---------|---------|-------------|
| `jobs` | `job`, `j` | Jobs (root) |
| `builds` | `build`, `b` | Builds |
| `queue` | `qu` | Build queue |
| `nodes` | `node`, `n`, `agents`, `agent` | Nodes / agents |
| `users` | `user`, `u` | Users |
| `credentials` | `cred`, `creds`, `cr` | Credentials |
| `plugins` | `plugin`, `pl` | Plugins |
| `views` | `view`, `v` | Jenkins views |
| `contexts` | `context`, `ctx` | Switch contexts |

### Path-aware navigation

Resource commands accept a path argument and push the matching nested view onto the stack (so `Esc` returns where you came from):

| Example | Result |
|---------|--------|
| `:jobs team-a/sub` | Jobs view scoped to folder `team-a/sub` |
| `:builds team-a/sub/deploy` | Builds for nested job `team-a/sub/deploy` |
| `:logs team-a/sub/deploy/42` | Console output of build #42 |
| `:tests team-a/sub/deploy/42` | JUnit test report (suites → cases → detail) for build #42 |
| `:reports team-a/sub/deploy/42` | HTML Publisher reports attached to build #42 |
| `:pipeline team-a/sub/deploy/42` | Blue Ocean pipeline graph for build #42 (aliases: `pipe`, `pl`, `bo`) |
| `:views team-a` | Jenkins views inside folder `team-a` |
| `:ctx prod` | Switch active Jenkins context to `prod` |

The prompt offers argument-aware autocomplete: while inside `team-a/sub`, typing `:builds d` ghosts to `:builds team-a/sub/deploy`. Suggestions come from whatever the current view exposes (jobs, contexts, view names), so a single Tab usually finishes the line.

### Other commands

| Command | Description |
|---------|-------------|
| `:url <jenkins-url>` | Connect ad-hoc to a Jenkins URL without editing config |
| `:bookmark` / `:bm` | List / save / jump bookmarks for the current view path |
| `:cache [clear\|stats]` | Inspect or wipe the HTTP cache |
| `:?`, `:h`, `:help` | Inline command cheatsheet |
| `:q`, `:q!`, `:qa`, `:quit`, `:exit` | Quit |

## Files & directories

| Path | Purpose |
|------|---------|
| `~/.config/j9s/config.yaml` | Main configuration |
| `~/.local/state/j9s/j9s.log` | Default log file (override with `--logFile`) |
| `bin/j9s` | Build artifact produced by `make build` |

## Testing

The repo ships a Jenkins mock server (`internal/client/mock`) with a fluent API for stubbing jobs, folders, builds, views and live-streaming log endpoints. It backs both client tests and view smoke tests.

```bash
make test                      # full suite
go test ./internal/view/...    # view + command-prompt unit tests
```

## License

Apache-2.0
