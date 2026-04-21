# j9s - Jenkins TUI Manager

A terminal UI for managing Jenkins instances, inspired by [k9s](https://github.com/derailed/k9s).

## Features

- **k9s-like interface**: Same TUI patterns, vim-like keybindings, and navigation
- **Multi-context support**: Manage multiple Jenkins instances
- **Resource views**: Jobs, Builds, Queue, Nodes, Users, Credentials, Plugins, Views
- **Actions**: Trigger builds, view logs, enable/disable jobs, and more
- **Authentication**: Supports API token, password, and OAuth/SSO

## Installation

```bash
go install github.com/roman-plevka/j9s@latest
```

Or build from source:

```bash
git clone https://github.com/roman-plevka/j9s.git
cd j9s
go build -o j9s .
```

## Configuration

Create a config file at `~/.j9s/config.yaml`:

```yaml
j9s:
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
      insecure: true  # Skip TLS verification
```

### Authentication Types

**API Token (recommended)**:
```yaml
auth:
  type: token
  username: your-username
  token: your-api-token
```

**Password**:
```yaml
auth:
  type: password
  username: your-username
  password: your-password
```

**OAuth/SSO**:
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
# Start with default context
j9s

# Start with specific context
j9s --context my-jenkins

# Start with specific view
j9s -c nodes

# Read-only mode
j9s --readonly
```

## Keybindings

### Global
| Key | Action |
|-----|--------|
| `:` | Command mode |
| `/` | Filter mode |
| `Esc` | Back / Clear |
| `Ctrl+C` | Quit |

### Navigation
| Key | Action |
|-----|--------|
| `Enter` | Select / Drill down |
| `j/k` or `↑/↓` | Navigate up/down |
| `g` | Go to top |
| `G` | Go to bottom |

### Jobs View
| Key | Action |
|-----|--------|
| `Enter` | View builds |
| `d` | Describe (show config) |
| `t` | Trigger build |
| `e` | Enable job |
| `D` | Disable job |
| `Ctrl+D` | Delete job |
| `r` | Refresh |

### Builds View
| Key | Action |
|-----|--------|
| `Enter` or `l` | View logs |
| `d` | Describe build |
| `s` | Stop build |
| `r` | Refresh |

### Logs View
| Key | Action |
|-----|--------|
| `/` | Filter logs |
| `g` | Go to top |
| `G` | Go to bottom |
| `w` | Toggle wrap |
| `s` | Toggle auto-scroll |
| `Esc` | Back |

## Commands

Type `:` to enter command mode, then:

| Command | Aliases | Description |
|---------|---------|-------------|
| `jobs` | `job`, `j` | View jobs |
| `builds` | `build`, `b` | View builds |
| `queue` | `q` | View build queue |
| `nodes` | `node`, `n`, `agents` | View nodes/agents |
| `users` | `user`, `u` | View users |
| `credentials` | `cred`, `creds`, `cr` | View credentials |
| `plugins` | `plugin`, `pl` | View plugins |
| `views` | `view`, `v` | View Jenkins views |
| `contexts` | `context`, `ctx` | Switch contexts |

## License

Apache-2.0
