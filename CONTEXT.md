# lazycloud — Context

## What it is

A terminal UI (TUI) application for browsing Google Cloud Platform (GCP) resources. It fetches resources via the GCP APIs on demand, caches them in a local SQLite database, and renders them in browsable tables. Think "lazygit/lazydocker for GCP".

## Tech stack

- Go 1.27 (module name: `lazycloud`)
- [tview v0.42.0](https://github.com/rivo/tview) — TUI widget toolkit
- [tcell v2 v2.8.1](https://github.com/gdamore/tcell) — terminal event layer
- `cloud.google.com/go/compute`, `cloud.google.com/go/resourcemanager`, `google.golang.org/api` — GCP SDK clients
- `modernc.org/sqlite` — pure-Go SQLite driver (no cgo)

## Layout

```
main.go               Thin entry point; calls `app.Run()`
pkg/
├── app/              All UI code (package app)
│   ├── run.go        App entry point; builds layout, installs input capture
│   ├── app.go        `app` struct, info table population, view switching
│   ├── handlers.go   Keyboard shortcuts, filter/command mode, overlay dismissal
│   ├── views.go      `viewDef` registry (per-view-kind definitions)
│   ├── state.go      `appState`, view kind constants
│   └── view_*.go     One file per resource view (see Views below)
cmd/
└── providers/        Cloud + cache logic
    ├── gcloud.go     GCP API calls + resource structs (VM, IPAddress, Disk, ...)
    ├── cache.go      SQLite cache (read-through persistence layer)
    ├── console.go    GCP Console URL builders (deep links)
    ├── log.go        Usage log (per resource type / project)
    ├── shortcuts.go  Numeric shortcut config file (`shortcuts.json`)
    └── state.go      `appState` (project ID, view kind) helpers
```

## Build & test

Task is the task runner (`Taskfile.yml`):

- `task run` — run the TUI locally (`go run .`)
- `task build` — build to `bin/lazycloud`
- `task install` — install to `GOPATH/bin`
- `task test` — `go test ./...`
- `task vet` — `go vet ./...`

Cache DB lives at `~/.config/lazycloud/cache.db`.

## Views

Each view is a `viewDef` struct (views.go) registered by view kind in `views` map (state.go). Fields: `title`, `headers`, `load` (read from cache), `refresh` (fetch from GCP API + save to cache), `selectRow` (Enter on a row), `copyRow` (copy JSON to clipboard), `consoleURL` (open GCP Console in browser).

| View kind | File | Table columns |
|---|---|---|
| `viewProjects` | view_projects.go | Project ID, Number, Updated |
| `viewVMs` / `viewVMsAll` | view_vms.go | Name, Zone, Machine Type, Status, Internal IPs, Updated (+ Project in All) |
| `viewIPs` / `viewIPsAll` | view_ips.go, view_ips_all.go | Name, Address, Region, Status, Updated (+ Project in All) |
| `viewNetworks` | view_networks.go | — also defines `formatUpdatedAt` helper |
| `viewRoutes` / `viewRoutesAll` | view_routes.go | Routes: Name, Network, Destination, Priority, Next Hop Type, Next Hop, Updated (+ Project in All) |
| `viewFirewalls` / `viewFirewallsAll` | view_firewalls.go | Firewall rules: Name, Network, Direction, Priority, Action (allow/deny proto:ports), Source, Targets, Updated (+ Project in All) |
| `viewBuckets` | view_buckets.go | Storage buckets |
| `viewAccounts` | view_accounts.go | Service accounts |
| `viewLBs` / `viewLBsAll` | view_loadbalancers.go | Load balancers: Name, IP:Port, Region, Protocol, Scheme, Backends (+ Project in All) |
| `viewClusters` / `viewClustersAll` | view_clusters.go | Kubernetes clusters: Name, Location, Status, Version, Node Pools, Created, Updated (+ Project in All) |
| `viewWorkloads` | view_workloads.go | Workloads of the cluster focused via `w` in the clusters views: Namespace, Kind, Name, Replicas, Images, Updated |
| `viewDNSZones` / `viewDNSZonesAll` | view_dnszones.go | DNS zones: Name, DNS Name, Visibility, Description, Updated (+ Project in All) |
| `viewDNSRecords` | view_dnsrecords.go | DNS records of the zone focused via `d` in the DNS zones views: Name, Type, TTL, Data, Updated |

### Workload fetching

GKE workloads are Kubernetes resources, so `ListClusterWorkloads` shells out: it writes credentials to a temp `KUBECONFIG` via `gcloud container clusters get-credentials` (zone vs region flag inferred from the location's dash count) and runs `kubectl get deployments,statefulsets,daemonsets --all-namespaces -o json`, parsing items into `Workload` (Replicas as `ready/total`). The workloads view is cluster-scoped: pressing `w` on a clusters-view row sets `appState.Cluster`/`appState.ClusterProject` and switches to it; there is no numeric shortcut or `:command`. Workloads are cached per `(project, cluster)` in the `workloads` table (rows carry their cluster reference) and `refresh` re-fetches only the focused cluster. Requires `gcloud` and `kubectl` on PATH. In the workloads view `g` opens the GKE workload overview filtered by namespace, `l` opens Logs Explorer with a `k8s_container` query (project/location/cluster/namespace/pod-app from the focused row), and `e` opens Logs Explorer with a `log_id("events")` + `resource.type="k8s_cluster"` query (cluster/location/project labels plus the workload name as a free-text match, via `GCPWorkloadEventsURL`).

### DNS zones and records

Cloud DNS zones are fetched via `ListProjectDNSZones` (ManagedZones list) and cached in `dns_zones`; records via `ListZoneDNSRecords(projectID, zone)` (ResourceRecordSets list), cached per `(project, zone)` in `dns_records`. The DNS zones view is reached via `:dns` (no numeric shortcut; all ten are taken). Pressing `d` on a zone row sets `appState.Zone`/`appState.ZoneProject` and switches to the zone-scoped records view, the same flow as clusters → workloads; `refresh` re-fetches only the focused zone, and `g` opens the Cloud DNS zone records page in the console.

### Load balancer composition

GCP LBs have no single resource: a row = one forwarding rule (`ForwardingRule`, with `TargetKind`/`Target` parsed from the rule's target URL via `targetRef`). `Cache.ResolveLoadBalancer(projectID, region, name)` composes the tree from cache: rule → target proxy (`TargetProxy`) → URL map (`URLMap` with `PathRules`) → backend services (`BackendService` with `Backends` + `HealthChecks`); internal/classic rules target a backend service directly. The fetchers (`ListProjectForwardingRules`, `ListProjectTargetProxies`, `ListProjectURLMaps`, `ListProjectBackendServices`) each save to their own table (`forwarding_rules`, `target_proxies`, `url_maps`, `backend_services`); refresh (`refreshProjectLBs` in view_loadbalancers.go) fetches all four. `selectRow` shows the `lb` details overlay with the full chain; the Backends column shows the default backend service plus `(+N)` extra path-rule services. `lbConsoleURL` picks the console deep link: `loadBalancers` list filtered by URL map name when the chain has a URL map, otherwise the `frontends` list filtered by the forwarding rule name.

Per-project views (`viewX`) use `appState.ProjectID`; the `viewXAll` variants cover all cached projects.

## UI architecture

- Single `tview.Table` (`a.info`) shared by all views. `populateInfoPanel()` (app.go) writes a bold fixed header row (`SetFixed(1, 0)`) + data rows; `applyFilter()` repaints with the active filter.
- Main layout (main.go): summary area (4 rows: project summary + numeric shortcuts panel, three key/desc pairs per row) / `tview.InputField` (`a.cmd`) / info table / status bar, wrapped in `tview.Pages` (`a.pages`).
- **Key handling** is two-layered:
  - Global: `a.inputCapture` (handlers.go) — `Ctrl-C` quits, `Esc` → `escapePressed()`, single-letter shortcuts (`shortcuts` table in handlers.go: `u` refresh, `y` copy row, `o` open console, `?` help, etc.). The `shortcuts` table is the single source of truth: the help overlay and the numeric summary panel (three key/desc pairs per row) both render from it. Numeric keys `1`–`9`, `0` switch views per `~/.config/lazycloud/shortcuts.json` (created with defaults on first run; keys map to view names — any `viewKind` string such as `projects`, `vms`, `dns-zones`, `workloads`, `vms-all`, …; loaded in `main` via `providers.LoadShortcuts` + `applyShortcutConfig` in shortcut_config.go, which rebuilds the numeric entries with desc `"show " + view title` and the action from `viewShortcutAction`). View-scoped: `w` (workloads of the focused cluster), `d` (DNS records of the focused zone, DNS zones views only), `b` (back to the parent view, workloads/DNS records views only — clusters or DNS zones, honoring `AllProjects`), `c` (fetch cluster credentials via `gcloud container clusters get-credentials`, clusters views only), `l` (logs, VM + clusters + node pools + workloads views, dispatches via `openLogs`; in the node pools view it opens Logs Explorer with a `log_id("events")` + `resource.type="k8s_node"` query plus the pool name as a free-text match, via `GCPNodePoolEventsURL`), `s` (IAP SSH, VM views), `r`/`x` (restore/delete project, projects view).
  - Enter on a table row fires `a.info.SetSelectedFunc` → `app.selected()` → the current view's `selectRow`.
- **Command vs filter mode**: the bottom InputField is either a filter (`/` prefix or after pressing the filter shortcut, Enter → `filterEnter()`) or a command (`:`, Enter → `commandEnter()`). `commandEnter()` special-cases typed IP literals (`net.ParseIP`) by jumping to the IPs view.

## Overlays (floating windows)

Built on `tview.Pages` (stacked, hidden/shown pages), centered with spacer-Flexes:

- `help` page — `newHelpOverlay()` (handlers.go), toggled by `?` (`toggleHelp()`).
- `vm` page — VM details window, opened via VM views' `selectRow` → `showVMDetails()` (view_vms.go). `vmDetails()` builds the `[yellow]label: [-]value` color-tagged text.
- `ip` page — IP address details window, opened via IP views' `selectRow` → `showIPDetails()` (view_ips.go). `ipDetails()` formats all cached `IPAddress` fields.
- `lb` page — Load balancer details window, opened via LB views' `selectRow` → `showLBDetails()` (view_loadbalancers.go). `lbDetails()` renders the composed rule → proxy → URL map → backend services chain.
- `dns` page — DNS record details window, opened via the DNS records view's `selectRow` → `showDNSRecordDetails()` (view_dnsrecords.go). `dnsRecordDetails()` renders all cached `DNSRecord` fields with one data value per line.
- `cluster` page — Cluster details window, opened via the clusters views' `selectRow` → `showClusterDetails()` (view_clusters.go). `clusterDetails()` renders all cached `Cluster` fields with one node pool per line (name, version, node count).
- All are dismissed by `Esc` — `escapePressed()` (handlers.go) removes the front page if it is `"vm"`, `"ip"`, `"lb"`, `"dns"`, `"cluster"`, or `"row"`, and refocuses the table.

**Pattern to add a new floating window**: use `a.newOverlay(title, content)` (app.go) — it builds the border+title dynamic-colors `TextView`, caps height to the terminal (`overlayHeight`: lines+2, at most screenHeight-2, min 3) and width (`overlayWidth`: 80, at most screenWidth-2, min 10; screen size captured from `SetBeforeDrawFunc` in main.go), and returns `(overlay, text)`. Then `a.pages.RemovePage(name)` + `AddPage(name, overlay, true, true)` + `a.tapp.SetFocus(text)` so arrows/PgUp/PgDn scroll long content; add the page name to the Esc dismissal check.

## Data model & cache

Resource structs live in gcloud.go (e.g. `IPAddress` has `ProjectID, Name, Address, AddressType, Status, Region, Subnetwork, Network, UpdatedAt`). Fetchers follow `ListProject<Res>(projectID)` naming (e.g. `ListProjectIPAddresses` returns global + regional addresses; `ListClusterWorkloads` is the shell-out exception). Short names for GCP resource URLs come from `resourceShortName`; load balancer target URLs are split into kind + name by `targetRef`.

The cache (cache.go) mirrors each struct to a SQLite table (e.g. `ip_addresses`) with upsert-on-conflict. Reads: `ListX` (one project), `ListAllX` (all), `GetX` (exact lookup, may return multiple rows — e.g. `GetIPAddresses` one per subnetwork); workloads are the cluster-scoped exception (`ListClusterWorkloads(projectID, cluster)`). Views always read from cache; `refresh` repopulates from the API in a goroutine and calls `QueueUpdateDraw` to repaint.

`console.go` builds GCP Console URLs per resource (`GCPVMURL`, `GCPAddressesURL`, `GCPVMLogsURL`, `GCPLoadBalancerURL`/`GCPLoadBalancerFrontendURL`, `GCPClusterURL`, `GCPWorkloadOverviewURL`, `GCPWorkloadLogsURL`, `GCPRoutesURL`, `GCPFirewallsURL`, `GCPDNSZoneURL`, ...) — used by the `o`/`g`/`l` shortcuts. Most list-page deep links encode a `pageState` filter (see `GCPAddressesURL`, `gcpLBListURL`).

## Conventions

- No comments in code; keep it that way.
- New view fields are optional function fields on `viewDef` — views without a behavior leave them nil.
- Tests live next to the code (`*_test.go` in both packages); run `task test` before considering work done.
- Background work (API fetches) uses `a.tapp.QueueUpdateDraw` to touch UI state; errors during fetch go through `a.reportError` (status + log), never panic.