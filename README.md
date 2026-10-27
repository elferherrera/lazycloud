# lazycloud

A terminal UI (TUI) for browsing Google Cloud Platform resources — think lazygit/lazydocker, but for GCP. It fetches resources on demand via the GCP APIs, caches them in a local SQLite database, and renders them in browsable tables.

Requires Go 1.27+ and an authenticated `gcloud` CLI. The workloads view additionally requires `kubectl` on your PATH.

## Getting started

### Option 1: mise (recommended)

[mise](https://mise.jdx.dev/) is a polyglot dev tool, task runner, and environment manager. Install it:

```sh
curl https://mise.run | sh
```

Then add it to your shell (restart afterwards):

```sh
echo 'eval "$(~/.local/bin/mise activate zsh)"' >> ~/.zshrc
```

mise reads the project's Go version requirement automatically. Install the tools and run the TUI:

```sh
mise use go@1.27   # pins the Go version in the project
mise exec -- go run .
```

Or install the [task](https://taskfile.dev) plugin to use the project's `Taskfile.yml` through mise:

```sh
mise plugin add task
mise task run
```

### Option 2: Task (or plain Go)

[Task](https://taskfile.dev) is a task runner used by this project (`Taskfile.yml`). Install it:

```sh
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin
```

(Or with Homebrew: `brew install go-task`.)

Make sure Go 1.27+ is installed, then use the tasks:

| Task | Description |
|---|---|
| `task run` | Run the TUI locally (`go run .`) |
| `task build` | Build the binary to `bin/lazycloud` |
| `task install` | Install the app to `$GOPATH/bin` |
| `task test` | Run the tests (`go test ./...`) |
| `task vet` | Run `go vet ./...` |

To install lazycloud permanently on your system:

```sh
task install        # then make sure $(go env GOPATH)/bin is on your PATH
```

Or with plain Go:

```sh
go install .
```

## Configuration files

All lazycloud files live under `~/.config/lazycloud/` (created on first run):

| File | Purpose |
|---|---|
| `cache.db` | SQLite cache of all fetched GCP resources |
| `shortcuts.json` | Numeric shortcut (`1`–`0`) to view mapping; created with defaults on first run |
| `last_project.json` | Last selected project, restored on startup |
| `lazycloud.log` | Usage / fetch error log |

## Shortcuts

Press `?` inside the TUI for the always up-to-date help overlay.

### Global

| Key | Action |
|---|---|
| `q` / `Ctrl-C` | Quit |
| `:` | Command mode (e.g. `:vms`, `:dns`, or type an IP address to jump to it) |
| `/` | Filter the current list |
| `u` | Refresh view (re-fetch from the GCP API and update the cache) |
| `y` | Copy focused row (JSON) to clipboard |
| `v` | Show focused row details |
| `g` | Open focused resource in GCP Console |
| `a` | Toggle all-projects view |
| `?` | Toggle help overlay |
| `Esc` | Dismiss overlay / leave filter or command mode |

### View switching (numeric, remappable via `shortcuts.json`)

| Key | View |
|---|---|
| `1` | Projects |
| `2` | VMs |
| `3` | Storage buckets |
| `4` | Service accounts |
| `5` | IP addresses |
| `6` | VPCs and subnets |
| `7` | Load balancers |
| `8` | Kubernetes clusters |
| `9` | Routes |
| `0` | Firewall rules |
| `:dns` | DNS zones (no numeric shortcut) |

### Context-sensitive

| Key | Action | Available in |
|---|---|---|
| `Enter` | Open details overlay for the focused row | everywhere |
| `w` | Show workloads for the focused cluster | clusters views |
| `n` | Show node pools for the focused cluster | clusters views |
| `m` | Show VMs for the focused node pool | node pools view |
| `c` | Fetch cluster credentials (`gcloud container clusters get-credentials`) | clusters views |
| `d` | Show DNS records for the focused zone | DNS zones views |
| `b` | Back to the previous view | workloads / DNS records / node pools views |
| `e` | Open workload events in GCP Console | workloads view |
| `l` | Open logs in Logs Explorer | VM / clusters / node pools / workloads views |
| `s` | SSH to VM via IAP | VM views |
| `r` | Restore last deleted project | projects view |
| `x` | Delete project from cache | projects view |
