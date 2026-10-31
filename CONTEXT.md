# lazycloud — Context

## What it is

A terminal UI (TUI) application for browsing Google Cloud Platform (GCP) resources. It fetches resources via the GCP APIs on demand, caches them in a local SQLite database, and renders them in browsable tables. Think "lazygit/lazydocker for GCP".

## Tech stack

- Go 1.27 (module name: `lazycloud`)
- [tview v0.42.0](https://github.com/rivo/tview) — TUI widget toolkit
- [tcell v2 v2.8.1](https://github.com/gdamore/tcell) — terminal event layer
- `cloud.google.com/go/compute`, `cloud.google.com/go/resourcemanager`, `google.golang.org/api` — GCP SDK clients
- [mmaid-go v0.6.1](https://github.com/aaronsb/mmaid-go) — renders mermaid diagrams as terminal art (routes next-hop chart)
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
    ├── theme.go      Color theme config file (`theme.json`)
    └── state.go      `appState` (project ID, view kind) helpers
```

## Build & test

mise manages tool versions and tasks (`mise.toml`):

- Go's version comes from go.mod, `mise install` installs it
- `mise run` runs the app; `mise run <task>` runs a task
- tasks: `build`, `install`, `test`, `vet`, `check` (vet + test)

Cache DB lives at `~/.config/lazycloud/cache.db`.

## Views

Each view is a `viewDef` struct (views.go) registered by view kind in `views` map (state.go). Fields: `title`, `headers`, `load` (read from cache), `refresh` (fetch from GCP API + save to cache), `selectRow` (Enter on a row), `resource` (row cells → underlying cached resource, used by `y` copy JSON, `v` row details and detail filtering), `samples` (zero-value resource structs used to validate detail filter paths), `consoleURL` (open GCP Console in browser).

| View kind | File | Table columns |
|---|---|---|
| `viewProjects` | view_projects.go | Project ID, Number, Updated |
| `viewVMs` / `viewVMsAll` | view_vms.go | Name, Zone, Machine Type, Status, Internal IPs, Updated (+ Project in All) |
| `viewIPs` / `viewIPsAll` | view_ips.go, view_ips_all.go | Name, Address, Region, Status, Updated (+ Project in All) |
| `viewNetworks` | view_networks.go | — also defines `formatUpdatedAt` helper |
| `viewRoutes` / `viewRoutesAll` | view_routes.go | Routes: Name, Network, Destination, Priority, Next Hop Type, Next Hop, Description, Updated (+ Project in All) |
| `viewFirewalls` / `viewFirewallsAll` | view_firewalls.go | Firewall rules: Name, Network, Direction, Priority, Action (allow/deny proto:ports), Source, Targets, Updated (+ Project in All) |
| `viewBuckets` | view_buckets.go | Storage buckets |
| `viewAccounts` | view_accounts.go | Service accounts |
| `viewLBs` / `viewLBsAll` | view_loadbalancers.go | Load balancers: Name, IP:Port, Region, Protocol, Scheme, Backends (+ Project in All) |
| `viewClusters` / `viewClustersAll` | view_clusters.go | Kubernetes clusters: Name, Location, Status, Version, Node Pools, Created, Updated (+ Project in All) |
| `viewWorkloads` | view_workloads.go | Workloads of the cluster focused via `w` in the clusters views: Namespace, Kind, Name, Replicas, Images, Updated |
| `viewDNSZones` / `viewDNSZonesAll` | view_dnszones.go | DNS zones: Name, DNS Name, Visibility, Description, Updated (+ Project in All) |
| `viewDNSRecords` | view_dnsrecords.go | DNS records of the zone focused via `d` in the DNS zones views: Name, Type, TTL, Data, Updated |
| `viewCloudRouters` / `viewCloudRoutersAll` | view_cloudrouters.go | Cloud routers: Name, Region, Network, ASN, Peers, NATs, Description, Updated (+ Project in All) |
| `viewVPNs` / `viewVPNsAll` | view_vpns.go | VPN gateways: Type, Name, Region, Network, IPs, Tunnels, Description, Updated (+ Project in All) |
| `viewInstanceGroups` / `viewInstanceGroupsAll` | view_instancegroups.go | Instance groups: Name, Location, Type, Size, Network, Subnetwork, Description, Updated (+ Project in All) |

### Workload fetching

GKE workloads are Kubernetes resources, so `ListClusterWorkloads` shells out: it writes credentials to a temp `KUBECONFIG` via `gcloud container clusters get-credentials` (zone vs region flag inferred from the location's dash count) and runs `kubectl get deployments,statefulsets,daemonsets --all-namespaces -o json`, parsing items into `Workload` (Replicas as `ready/total`). The workloads view is cluster-scoped: pressing `w` on a clusters-view row sets `appState.Cluster`/`appState.ClusterProject` and switches to it; there is no numeric shortcut or `:command`. Workloads are cached per `(project, cluster)` in the `workloads` table (rows carry their cluster reference) and `refresh` re-fetches only the focused cluster. Requires `gcloud` and `kubectl` on PATH. In the workloads view `g` opens the GKE workload overview filtered by namespace, `l` opens Logs Explorer with a `k8s_container` query (project/location/cluster/namespace/pod-app from the focused row), `e` opens Logs Explorer with a `log_id("events")` + `resource.type="k8s_cluster"` query (cluster/location/project labels plus the workload name as a free-text match, via `GCPWorkloadEventsURL`), and `c` opens a shell with the kubectl context set (`openWorkloadShell` in view_workloads.go): the kubeconfig context name is resolved first via `kubectl config get-contexts -o name` (`kubectlClusterContext` → `findClusterContextName`: exact `gke_<project>_<location>_<cluster>` built from the cached cluster, else the first context ending in `_<cluster>` preferring ones containing the project, else the first containing the cluster name; no match aborts with a hint to fetch credentials first), then the TUI suspends, `kubectl config set-context <context> --namespace <namespace>` runs in the terminal (namespace from the focused row — in the user's default kubeconfig, not the fetcher's temp one), then an interactive `$SHELL` (fallback `/bin/sh`) takes over the terminal and stays open (`workloadShellCommand`: `LAZYCLOUD_CONTEXT=true` is added to its environment so nested commands can detect the lazycloud shell; the var exists only inside the shell — `checkNestedExecution` at the top of `Run` makes lazycloud refuse to start while it is `true`, returning "Detected nested lazycloud execution. Exit this context to start lazycloud"); exiting the shell resumes the app loop, like `s` IAP SSH.

### DNS zones and records

Cloud DNS zones are fetched via `ListProjectDNSZones` (ManagedZones list) and cached in `dns_zones`; records via `ListZoneDNSRecords(projectID, zone)` (ResourceRecordSets list), cached per `(project, zone)` in `dns_records`. The DNS zones view is reached via `:dns` (no numeric shortcut; all ten are taken). Pressing `d` on a zone row sets `appState.Zone`/`appState.ZoneProject` and switches to the zone-scoped records view, the same flow as clusters → workloads; `refresh` re-fetches only the focused zone, and `g` opens the Cloud DNS zone records page in the console.

### Cloud routers

Cloud Routers are fetched via `ListProjectCloudRouters` (Routers aggregated list; `BgpPeers` → `CloudRouterPeer` with interface/IP/ASN, `Nats` → names, `Bgp.Asn` → `BGPASN`) and cached in `cloud_routers` keyed by `(project, region, name)`, with `Peers`/`NATs` stored as JSON like VM interfaces. The view is reached via `:routers` (`cloud-routers` alias works too; no numeric shortcut). The table shows the router ASN with `Peers`/`NATs` counts; `y`/`v` expose the full peer list via the cached `CloudRouter` JSON, and `g` opens the Cloud Router details page (`GCPCloudRouterURL`, region-scoped like the cluster deep link).

### VPNs

VPN rows are gateways of both kinds: `ListProjectVPNGateways` fetches HA VPN gateways, classic target VPN gateways and all VPN tunnels, grouping the tunnels by gateway self-link (`VpnGateway` for HA, `TargetVpnGateway` for classic) into `Tunnels` (`VPNTunnel` with interface/peer IP/IKE/status/router; the Router short name matches rows in the cloud routers view). Everything is cached in `vpn_gateways` keyed by `(project, region, name)`, `IPs` (HA interface addresses) and `Tunnels` stored as JSON. The view is reached via `:vpns` (`vpn` alias works too; no numeric shortcut). The Type column is `ha`/`classic` like the networks view's Type column; classic rows have no IPs (classic gateway IPs live on their forwarding rules). `y`/`v` expose the full tunnel list via the cached `VPNGateway` JSON, and `g` opens the VPN gateway details page (`GCPVPNGatewayURL`, region-scoped).

### Instance groups

Rows are instance groups of both kinds: `ListProjectInstanceGroups` fetches managed groups (`instanceGroupManagers` aggregated list → Type `managed`, size = target size, template short name) and unmanaged groups (`instanceGroups` aggregated list → Type `unmanaged`, size = member count, network/subnetwork); a managed group also exposes a same-named underlying instance group, so the unmanaged pass skips `(zone, name)` keys claimed by a manager and the managed row inherits its network/subnetwork. Groups can be zonal or regional — the Location column holds the zone or region short name. Everything is cached in `instance_groups` keyed by `(project, zone, name)`. The view is reached via `:igs` (`instance-groups` alias works too; no numeric shortcut). `y`/`v` expose the cached `InstanceGroup` JSON, and `g` opens the instance groups details page (`GCPInstanceGroupURL`). Pressing `m` on a group row switches to the VMs view filtered to the group's members: `ListInstanceGroupVMs(projectID, location, name)` calls `instanceGroups.listInstances` (zonal vs regional client inferred from the location's dash count, like `ListClusterWorkloads`), members are matched against the cached VMs by `(zone, name)` via `vmsInInstanceGroup`, VMs are fetched first if not cached, and Esc clears the filter — the same flow as node pools' `m`.

### Routes next-hop chart

Pressing `c` on a routes-view row (`showRouteHopChart` in view_routes.go, both `viewRoutes` and `viewRoutesAll`) shows the `route` overlay: a mermaid `graph TD` chart — route box → hop diamond (kind: value, destination range as edge label) — rendered as terminal art with `mmaid.Render` (mmaid-go). When the next hop parses as an IP literal (`net.ParseIP`), `routeHopTargets` matches it against the route's project in cache: cached VMs by interface InternalIP/ExternalIP (`vmMatchesIP`, box per VM: `VM: name (zone)`) and cached forwarding rules by IPAddress (rounded box per LB: `LB: name (region, scheme)`); an IP hop with no match renders a dotted edge to an `unknown` box, non-IP hops stop at the hop node. Each LB target continues via `lbBackendsMermaid` + `Cache.ResolveLoadBalancer`: one `fwr: name` box per resolved backend service (URL-map path-rule services included), and one hexagon per backend printing its group name (`group: <Backend.Group>`, already a short name from `resourceShortName`); services or groups missing from cache simply end the branch. `renderMermaidArt` pins the render width by pointing `os.Stdout`/`os.Stderr` at a pipe (so mmaid's TIOCGWINSZ ioctl fails and it falls back to `COLUMNS=80`) and restores both files plus the env afterwards — otherwise the layout stretches to ~2× the real terminal width and overflows the overlay. The overlay uses `a.newArtOverlay` (app.go): like `newOverlay` but `SetWrap(false)` and sized to the art (`artOverlayWidth`: widest line + 2, capped to screenWidth-2, min 10) instead of the fixed 80.

### Load balancer composition

GCP LBs have no single resource: a row = one forwarding rule (`ForwardingRule`, with `TargetKind`/`Target` parsed from the rule's target URL via `targetRef`). `Cache.ResolveLoadBalancer(projectID, region, name)` composes the tree from cache: rule → target proxy (`TargetProxy`) → URL map (`URLMap` with `PathRules`) → backend services (`BackendService` with `Backends` + `HealthChecks`); internal/classic rules target a backend service directly. The fetchers (`ListProjectForwardingRules`, `ListProjectTargetProxies`, `ListProjectURLMaps`, `ListProjectBackendServices`) each save to their own table (`forwarding_rules`, `target_proxies`, `url_maps`, `backend_services`); refresh (`refreshProjectLBs` in view_loadbalancers.go) fetches all four. `selectRow` shows the `lb` details overlay with the full chain; the Backends column shows the default backend service plus `(+N)` extra path-rule services. `lbConsoleURL` picks the console deep link: `loadBalancers` list filtered by URL map name when the chain has a URL map, otherwise the `frontends` list filtered by the forwarding rule name.

Per-project views (`viewX`) use `appState.ProjectID`; the `viewXAll` variants cover all cached projects.

## UI architecture

- Single `tview.Table` (`a.info`) shared by all views. `populateInfoPanel()` (app.go) writes a bold fixed header row (`SetFixed(1, 0)`) + data rows; `applyFilter()` repaints with the active filter.
- Main layout (main.go): summary area (4 rows: project summary + numeric shortcuts panel, three key/desc pairs per row) / `tview.InputField` (`a.cmd`) / info table / status bar, wrapped in `tview.Pages` (`a.pages`).
- **Theme**: `~/.config/lazycloud/theme.json` (created with the standard layout on first run and migrated in place when keys are missing — absent `shortcutLabels`/`shortcutText`/`summaryLabels`/`summaryText`/`statusText`/`detailText`/`activeRowBackground` inherit the file's `labels`/`text` while `statusLabels`, `detailLabels` and `activeRowText` keep their standard values, loaded in `Run` via `providers.LoadTheme` + `applyTheme` in pkg/app/theme.go, before any tview primitive is created because tview captures the style colors at widget creation) sets fourteen color selectors, with invalid color names falling back to the standard values: `labels` (the yellow accents — info table headers → `tview.Styles.SecondaryTextColor`), `text` (the regular text of those sections → `tview.Styles.PrimaryTextColor`), `borders` (info table and overlay borders → `tview.Styles.BorderColor`), `title` (info table and overlay border titles → `tview.Styles.TitleColor`), `shortcutLabels`/`shortcutText` (key and description cells of the shortcuts panel, help overlay and numeric summary panel, set explicitly via `theme.shortcutLabelColor()`/`theme.shortcutTextColor()` in `populateShortcutsPanel`/`newShortcutsSummaryPanel`), `statusLabels`/`statusText` (the status bar: `statusLabels` colors the informational messages written via `a.statusInfo` — per-view `def.status` row-count strings, fetching/refresh notices and the filter-mode hint — applied as `theme.statusLabelTag()`; `statusText` is the status TextView's base color for untagged segments like error details, set via `SetTextColor` in `Run`. Error and success messages stay hardcoded `[red]`/`[green]`.), `summaryLabels`/`summaryText` (the project summary TextView: prefixes and values via `theme.summaryLabelTag()`/`theme.summaryTextTag()` in `renderSummary`/`newSummaryPanel`, base color for untagged segments like the error details of `reportError` via `SetTextColor`; summary error prefixes stay hardcoded `[red]`), `detailLabels`/`detailText` (the details floating windows: `detailLabels` colors the field names in the `*Details` builders (`vmDetails`, `ipDetails`, `lbDetails`, `clusterDetails`, `dnsRecordDetails`) and `appendRowDetails` via `theme.detailLabelTag()`; `detailText` is the overlay TextView's base color for the values, set via `SetTextColor` in `newOverlay` — the help overlay and the route art overlay don't use `newOverlay` and are unaffected), and `activeRowText`/`activeRowBackground` (the info table's selected row, set as `SetSelectedStyle` in `newInfoPanel` via `theme.activeRowTextColor()`/`theme.activeRowBackgroundColor()` — without them tview's default swaps the cell colors, so the background tracks `text` and the text stays black; the standard values `black`/`white` render identically to that swap under the standard theme).
- **Key handling** is two-layered:
  - Global: `a.inputCapture` (handlers.go) — `Ctrl-C` quits, `Esc` → `escapePressed()`, single-letter shortcuts (`shortcuts` table in handlers.go: `u` refresh, `y` copy row, `o` open console, `?` help, etc.). The `shortcuts` table is the single source of truth: the help overlay and the numeric summary panel (three key/desc pairs per row) both render from it. Numeric keys `1`–`9`, `0` switch views per `~/.config/lazycloud/shortcuts.json` (created with defaults on first run; keys map to view names — any `viewKind` string such as `projects`, `vms`, `dns-zones`, `workloads`, `vms-all`, …; loaded in `main` via `providers.LoadShortcuts` + `applyShortcutConfig` in shortcut_config.go, which rebuilds the numeric entries with desc `"show " + view title` and the action from `viewShortcutAction`). View-scoped: `w` (workloads of the focused cluster), `d` (DNS records of the focused zone, DNS zones views only), `b` (back to the parent view, workloads/DNS records views only — clusters or DNS zones, honoring `AllProjects`), `c` (fetch cluster credentials via `gcloud container clusters get-credentials`, clusters views only; in the routes views `c` instead shows the next-hop chart — see Routes next-hop chart; in the workloads view `c` instead sets the kubectl context and opens an interactive shell — see Workload fetching), `l` (logs, VM + clusters + node pools + workloads views, dispatches via `openLogs`; in the node pools view it opens Logs Explorer with a `log_id("events")` + `resource.type="k8s_node"` query plus the pool name as a free-text match, via `GCPNodePoolEventsURL`), `s` (IAP SSH, VM views), `r`/`x` (restore/delete project, projects view).
  - Enter on a table row fires `a.info.SetSelectedFunc` → `app.selected()` → the current view's `selectRow`.
- **Command vs filter mode**: the bottom InputField is either a filter (`/` prefix or after pressing the filter shortcut, Enter → `filterEnter()`) or a command (`:`, Enter → `commandEnter()`). `commandEnter()` special-cases typed IP literals (`net.ParseIP`) by jumping to the IPs view. Filter text is a case-insensitive substring match across all cells; a `-` followed by a space starts an exclusion term (`splitFilterQuery` in app.go, term parsing/matching in filter.go): text before the first `- ` is the include term, each segment after a `- ` is an exclusion that hides rows matching it, e.g. `web - staging - canary`. Empty exclusion segments (mid-typing) are ignored; a dash without a following space is matched literally. A term whose first token (before the first space) is a field path of the view's resource is a **detail filter**: `Labels env=prod` matches only the Labels detail, `ServiceAccounts.*.Email x` matches any element of the ServiceAccounts list by Email, `DataDisks.*.SizeGB 100` by SizeGB. Paths are `.`-separated (`Field`, `Field.*.Sub` where `*` descends into a list/map matching any element, `Labels.key` for map keys); field names match case-insensitively with a singular/plural alias (`ServiceAccount` ≡ `ServiceAccounts`). A term is a detail filter only when its path validates against the view's `samples` and has a space (`env prod` in the VMs view stays a plain substring); resolved resources are cached per `filterBase` in `a.rowResources` (reset by `a.setFilterBase`, built lazily by `resolveRowResources` only when a detail term is present) and matching never touches row cells for detail terms. Values render as strings (`k=v` joined by `,` for maps/sorted structs), so `Labels env=prod` matches the `env=prod` pair. In filter mode, Tab/Backtab cycle the view's available detail paths (`detailPaths` in filter.go reflects over `samples`; struct fields, `*` for list-of-struct element descent, maps/scalar slices terminal, sorted): `completeFilterDetail` (handlers.go) completes a partial head (the text before the first space, case-insensitive prefix of a path) to the first (Tab) or last (Backtab) matching path, prepends the first/last path when the head matches nothing, rotates a head that already equals a path to the next/previous one keeping the rest, and drops the prefix when rotating past the last/first path. The hook lives in `inputCapture` before the cmd-focus early return, so Tab works both while typing and in refinement mode (focus on the table); `SetText` fires `changed` → `applyFilter`, so each rotation re-filters live.

## Overlays (floating windows)

Built on `tview.Pages` (stacked, hidden/shown pages), centered with spacer-Flexes:

- `help` page — `newHelpOverlay()` (handlers.go), toggled by `?` (`toggleHelp()`).
- `vm` page — VM details window, opened via VM views' `selectRow` → `showVMDetails()` (view_vms.go). `vmDetails()` builds the `[yellow]label: [-]value` color-tagged text.
- `ip` page — IP address details window, opened via IP views' `selectRow` → `showIPDetails()` (view_ips.go). `ipDetails()` formats all cached `IPAddress` fields.
- `lb` page — Load balancer details window, opened via LB views' `selectRow` → `showLBDetails()` (view_loadbalancers.go). `lbDetails()` renders the composed rule → proxy → URL map → backend services chain.
- `dns` page — DNS record details window, opened via the DNS records view's `selectRow` → `showDNSRecordDetails()` (view_dnsrecords.go). `dnsRecordDetails()` renders all cached `DNSRecord` fields with one data value per line.
- `cluster` page — Cluster details window, opened via the clusters views' `selectRow` → `showClusterDetails()` (view_clusters.go). `clusterDetails()` renders all cached `Cluster` fields with one node pool per line (name, version, node count).
- `route` page — Routes next-hop chart window, opened via `c` in the routes views → `showRouteHopChart()` (view_routes.go). See Routes next-hop chart.
- All are dismissed by `Esc` — `escapePressed()` (handlers.go) removes the front page if it is `"vm"`, `"ip"`, `"lb"`, `"dns"`, `"cluster"`, `"route"`, or `"row"`, and refocuses the table.

**Pattern to add a new floating window**: use `a.newOverlay(title, content)` (app.go) — it builds the border+title dynamic-colors `TextView`, caps height to the terminal (`overlayHeight`: lines+2, at most screenHeight-2, min 3) and width (`overlayWidth`: 80, at most screenWidth-2, min 10; screen size captured from `SetBeforeDrawFunc` in main.go), and returns `(overlay, text)`. For pre-rendered art that must not wrap, `a.newArtOverlay` is the same pattern with `SetWrap(false)` and width sized to the content (`artOverlayWidth`). Then `a.pages.RemovePage(name)` + `AddPage(name, overlay, true, true)` + `a.tapp.SetFocus(text)` so arrows/PgUp/PgDn scroll long content; add the page name to the Esc dismissal check.

## Data model & cache

Resource structs live in gcloud.go (e.g. `IPAddress` has `ProjectID, Name, Address, AddressType, Status, Region, Subnetwork, Network, UpdatedAt`). Fetchers follow `ListProject<Res>(projectID)` naming (e.g. `ListProjectIPAddresses` returns global + regional addresses; `ListClusterWorkloads` is the shell-out exception). Short names for GCP resource URLs come from `resourceShortName`; load balancer target URLs are split into kind + name by `targetRef`.

The cache (cache.go) mirrors each struct to a SQLite table (e.g. `ip_addresses`) with upsert-on-conflict. Reads: `ListX` (one project), `ListAllX` (all), `GetX` (exact lookup, may return multiple rows — e.g. `GetIPAddresses` one per subnetwork); workloads are the cluster-scoped exception (`ListClusterWorkloads(projectID, cluster)`). Views always read from cache; `refresh` repopulates from the API in a goroutine and calls `QueueUpdateDraw` to repaint.

`console.go` builds GCP Console URLs per resource (`GCPVMURL`, `GCPAddressesURL`, `GCPVMLogsURL`, `GCPLoadBalancerURL`/`GCPLoadBalancerFrontendURL`, `GCPClusterURL`, `GCPWorkloadOverviewURL`, `GCPWorkloadLogsURL`, `GCPRoutesURL`, `GCPFirewallsURL`, `GCPDNSZoneURL`, ...) — used by the `o`/`g`/`l` shortcuts. Most list-page deep links encode a `pageState` filter (see `GCPAddressesURL`, `gcpLBListURL`).

## Conventions

- No comments in code; keep it that way.
- New view fields are optional function fields on `viewDef` — views without a behavior leave them nil.
- Tests live next to the code (`*_test.go` in both packages); run `mise run test` before considering work done.
- Background work (API fetches) uses `a.tapp.QueueUpdateDraw` to touch UI state; errors during fetch go through `a.reportError` (status + log), never panic.