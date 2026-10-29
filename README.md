# lazycloud

A terminal UI (TUI) for browsing Google Cloud Platform resources — think lazygit/lazydocker, but for GCP. It fetches resources on demand via the GCP APIs, caches them in a local SQLite database, and renders them in browsable tables.

Requires Go 1.27+ and an authenticated `gcloud` CLI. The workloads view additionally requires `kubectl` on your PATH.

## Installation

### mise (recommended)

Install [mise](https://mise.jdx.dev/installing-mise.html), with its shims directory on your `PATH`, then:

```sh
mise use -g go:github.com/elferherrera/lazycloud
```

### Go

```sh
go install github.com/elferherrera/lazycloud@latest
```

This puts the binary in `$(go env GOPATH)/bin`, so make sure that directory is on your `PATH`.

## Development

Install [mise](https://mise.jdx.dev/installing-mise.html), then:

```sh
git clone https://github.com/elferherrera/lazycloud.git
cd lazycloud
mise run
```

mise reads the required Go version from `go.mod`, so no manual toolchain setup is needed. Available tasks:

| Command | Description |
|---|---|
| `mise run` | Run the TUI locally (`go run .`) |
| `mise run build` | Build the binary to `./lazycloud` |
| `mise run install` | Install the binary to `GOBIN` (on `PATH`) |
| `mise run test` | Run the tests (`go test ./...`) |
| `mise run vet` | Run `go vet ./...` |
| `mise run check` | Run `vet` and `test` |

Run `mise tasks` to list all tasks.

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

### View switching (command mode)

| Command | View |
|---|---|
| `:projects` | Projects |
| `:vms` | VMs |
| `:buckets` | Storage buckets |
| `:accounts` / `:sa` | Service accounts |
| `:ips` | IP addresses (a raw IP like `:10.0.0.5` jumps here too) |
| `:networks` / `:vpcs` | VPCs and subnets |
| `:lbs` / `:loadbalancers` | Load balancers |
| `:clusters` / `:gke` | Kubernetes clusters |
| `:routes` | Routes |
| `:firewalls` | Firewall rules |
| `:dns` / `:zones` | DNS zones |
| `:routers` / `:cloud-routers` | Cloud routers |
| `:vpns` / `:vpn` | VPN gateways |

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
