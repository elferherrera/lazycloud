# lazycloud

A terminal UI (TUI) for browsing Google Cloud Platform resources — think lazygit/lazydocker, but for GCP. It fetches resources on demand via the GCP APIs, caches them in a local SQLite database, and renders them in browsable tables.

Requires Go 1.27+ and an authenticated `gcloud` CLI. The workloads view additionally requires `kubectl` on your PATH, and its `c` shortcut requires the `SHELL` environment variable to be set to open its interactive shell.

## Installation

### Homebrew (recommended)

On macOS or Linux with [Homebrew](https://brew.sh):

```sh
brew tap elferherrera/tap https://github.com/elferherrera/homebrew-tap
brew install elferherrera/tap/lazycloud
```

The formula covers macOS (Apple Silicon and Intel) and Linux x86_64, pulls the same release binaries as `go install`, and `brew upgrade` picks up new versions: a `v*` tag push builds and tests all platforms, publishes the tarballs with `.sha256` sidecars as a GitHub release, and regenerates the formula in the tap from those sidecars.

### mise

```sh
mise use -g go:github.com/elferherrera/lazycloud
```

### Go

```sh
go install github.com/elferherrera/lazycloud@latest
```

This puts the binary in `$(go env GOPATH)/bin`, so make sure that directory is on your `PATH`.

### Prebuilt binaries

Each release ships `lazycloud-darwin-arm64.tar.gz`, `lazycloud-darwin-x86_64.tar.gz` and `lazycloud-linux-x86_64.tar.gz` with `.sha256` sidecars on the [releases page](https://github.com/elferherrera/lazycloud/releases); unpack and put `lazycloud` on your `PATH`.

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
| `theme.json` | Layout colors — `labels` (yellow accents: info headers), `text` (regular text), `borders` (info panel and overlay borders), `title` (border titles), `shortcutLabels` (shortcut keys; falls back to `labels`), `shortcutText` (shortcut descriptions; falls back to `text`), `statusLabels` (status bar info messages; standard yellow), `statusText` (status bar plain text; falls back to `text`), `summaryLabels` (summary prefixes; falls back to `labels`), `summaryText` (summary values; falls back to `text`), `detailLabels` (details overlay field names; standard yellow), `detailText` (details overlay values; falls back to `text`), `activeRowText` (active row text; standard black), `activeRowBackground` (active row background; falls back to `text`); status errors/successes and summary errors stay red/green. Created with the standard layout on first run, missing keys added in place on load (shortcut/summary selectors, `statusText`, `detailText` and `activeRowBackground` inherit `labels`/`text`). Any tcell color name (e.g. `aqua`, `orange`) or `#rrggbb` hex |
| `last_project.json` | Last selected project, restored on startup |
| `lazycloud.log` | Usage / fetch error log |

## Shortcuts

Press `?` inside the TUI for the always up-to-date help overlay.

### Global

| Key | Action |
|---|---|
| `q` / `Ctrl-C` | Quit |
| `:` | Command mode (e.g. `:vms`, `:sort asc name`, or type an IP address to jump to it) |
| `/` | Filter the current list (see [Filtering](#filtering)) |
| `S` / `:search` | Search side panel across every view and project (see [Search](#search)) |
| `u` | Refresh view (re-fetch from the GCP API and update the cache) |
| `U` | Refresh every view of the current project, one after another (the status bar shows the view being fetched) |
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
| `:igs` / `:instance-groups` | Instance groups |

### Commands (command mode)

Anything with arguments or not matching a view name is treated as a command; unknown commands report an error in the status bar.

| Command | Effect |
|---|---|
| `:sort asc <column>` / `:sort desc <column>` | Sort the currently visible rows by a column (the active filter, if any, stays active). The column name matches the view headers case-insensitively, spaces allowed (e.g. `:sort asc name`, `:sort desc machine type`) |
| `:write <file>` | Write the currently visible rows to a file in the current folder: `*.csv` writes the table (headers + visible columns), `*.json` / `*.yaml` write the full resource details of every visible element |

### Context-sensitive

| Key | Action | Available in |
|---|---|---|
| `Enter` | Open details overlay for the focused row | everywhere |
| `w` | Show workloads for the focused cluster | clusters views |
| `n` | Show node pools for the focused cluster | clusters views |
| `m` | Show VMs for the focused node pool | node pools view |
| `m` | Show VMs for the focused instance group | instance groups views |
| `c` | Fetch cluster credentials (`gcloud container clusters get-credentials`) | clusters views |
| `c` | Show next-hop chart (rendered mermaid) for focused route; an IP next hop is matched against cached VMs and load balancers, and load balancers continue into their backend services and groups | routes views |
| `c` | Find the kubeconfig context whose name matches the focused row's cluster (via `kubectl config get-contexts`; gcloud names contexts `gke_<project>_<location>_<cluster>`) and set its namespace (`kubectl config set-context <context> --namespace <namespace>`), then open an interactive shell; exiting the shell returns to lazycloud — requires the `SHELL` environment variable to be set | workloads view |
| `d` | Show DNS records for the focused zone | DNS zones views |
| `b` | Back to the previous view | workloads / DNS records / node pools views |
| `e` | Open workload events in GCP Console | workloads view |
| `l` | Open logs in Logs Explorer | VM / clusters / node pools / workloads views |
| `s` | SSH to VM via IAP | VM views |
| `r` | Restore last deleted project | projects view |
| `x` | Delete project from cache | projects view |

## Search

Press `S` (not in the VMs view) or run `:search` to open a search side panel that searches every indexed view across every project. `/` stays the per-view filter.

The index is built over the local cache in the background, most-used views first (VMs, IPs, VPCs, buckets, ...), so results appear progressively while the rest indexes. Views with nothing cached are pulled from the GCP API in the background while the search is open, and results keep updating as the data arrives.

Results group by resource kind, and each group lists its projects as indented subrows with per-project match counts. Moving the selector previews in the main view: a kind header switches the main panel to that resource with the query as the live filter, and a project subrow also adds the project to the filter.

| Key | Action |
|---|---|
| type | Live results, rows capped at 100 per kind (the header shows `Kind (N)` with the total) |
| `Tab` | Toggle focus between the panel and the main view (the panel border highlights while focused) |
| `Enter` | Land the previewed view and filter, keeping the panel open as the sidebar |
| `Esc` | Close the search (the query is kept for the next open) |

Terms work like the filter: space-separated substrings, all of which must match, `-x` excludes. Matching is case-insensitive across the whole row.

## Filtering

Press `/` to filter the current list. Matching is case-insensitive and the list repaints on every keystroke.

| Term | Matches |
|---|---|
| `web` | any row with a cell containing `web` (substring across all columns) |
| `web - staging - canary` | `web`, excluding rows matching `staging` or `canary` |
| `Labels env=prod` | rows whose labels contain the `env=prod` pair |
| `Labels.env prod` | rows whose `env` label contains `prod` |
| `ServiceAccounts.*.Email vm-sa` | rows with any service account whose email contains `vm-sa` |
| `DataDisks.*.SizeGB 100` | rows with any data disk whose size contains `100` |
| `web + running` | rows matching both `web` and `running` |
| `europe - Labels env=prod` | `europe`, excluding rows labeled `env=prod` |

A term is a **detail filter** when its first token (before the first space) is a field path of the view's resource: it matches only that field, not the visible cells. Paths are `.`-separated, descend lists with `*` (any element), and reach map values via their key (`Labels.env`). Field names are case-insensitive with a singular/plural alias (`ServiceAccount` ≡ `ServiceAccounts`), and if the first token is not a valid field path the whole term falls back to a plain substring match.

A `+ ` followed by a space starts an **include term**: a row is only shown when it matches every include term, so `web + running` requires both (`Labels env=prod + running` combines detail and plain includes too). Terms can be combined with `- ` exclusions (`europe + running - other`). A `+` without a following space is matched literally, like a substring.

`Tab` completes and cycles detail paths: a partial path completes to the full one (`Servi` → `ServiceAccounts`), further presses rotate through every available path of the view, and text that matches no path gets the first path prepended. `Backtab` cycles backwards. `Enter` applies the filter and returns focus to the table (press `/` again to narrow further within the visible rows); `Esc` clears the filter.

## Command line

Running `lazycloud` with no arguments still opens the TUI. Passing a subcommand runs the same views as an agent-friendly CLI instead: it reads the same cache, refreshes with the same fetchers, and prints snake_case JSON by default.

```
lazycloud list <resource> [--project <id>] [--json|--yaml|--table|--ndjson] [--refresh]
lazycloud get <resource> <selector...> [--json|--yaml|--table|--ndjson]
lazycloud projects            # shortcut: lazycloud list projects
lazycloud resources           # the valid resource names
lazycloud --help              # the full CLI surface: subcommands, flags, resources, selectors
```

Resources: `projects`, `vms`, `disks`, `clusters`, `networks`, `buckets`, `serviceaccounts` (`sa`), `iam`. Singular aliases work (`lazycloud get vm ...`).

Selectors: `project` for projects, `project/zone/name` for VMs and disks, `project/name` for clusters, buckets, and VPCs, `project/region/name` for subnets, `project/email` for service accounts, `project/member/role` for IAM bindings (the role keeps its slashes).

Examples:

```
$ lazycloud list vms --project my-prod --refresh
[{"project_id":"my-prod","name":"api-01","zone":"europe-west1-b",...}]

$ lazycloud get vm my-prod/europe-west1-b/api-01
{"project_id":"my-prod","name":"api-01","zone":"europe-west1-b",...}

$ lazycloud list clusters --project my-prod --table
Name        Location      Status    Version  Node Pools   Created             Updated
cluster-01  europe-west1  RUNNING   1.31     pool-1 (+1)  2026-01-01 00:00    2026-01-02 10:00
```

Behavior:

- Output formats: `--json` (default, compact array), `--ndjson` (one object per line), `--yaml`, `--table` (the same columns as the TUI view).
- Keys are snake_case; map keys like VM labels pass through untouched.
- `--project <id>` scopes one project; without it (and without `LAZYCLOUD_PROJECT`) the list covers every project in the cache. `--refresh` re-fetches from GCP first, like the TUI's `u` key, and records the view update so the TUI's staleness display stays accurate. The global `projects` resource rejects a project scope (`--project` or `LAZYCLOUD_PROJECT`) with a usage error instead of ignoring it.
- Unscoped `--table` output prints the TUI's all-projects columns, including the trailing Project column; a scoped `--table` prints the per-project view's columns.
- stdout is data only; warnings and diagnostics go to stderr.
- Exit codes: `0` ok (empty result is `[]`), `1` usage, `2` not found, `3` infrastructure (cache, auth, quota, API).

Design rationale: [docs/agent-cli-proposal.md](docs/agent-cli-proposal.md).
