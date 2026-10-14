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
cmd/
├── tui/          All UI code (package main)
│   ├── main.go         App entry point; builds layout, installs input capture
│   ├── app.go          `app` struct, info table population, view switching
│   ├── handlers.go      Keyboard shortcuts, filter/command mode, overlays dismissal
│   ├── views.go         `viewDef` registry (per-view-kind definitions)
│   ├── state.go         `appState`, view kind constants
│   ├── view_*.go        One file per resource view (see Views below)
├── providers/    Cloud + cache logic
│   ├── gcloud.go       GCP API calls + resource structs (VM, IPAddress, Disk, ...)
│   ├── cache.go        SQLite cache (read-through persistence layer)
│   ├── console.go      GCP Console URL builders (deep links)
│   ├── log.go          Usage log (per resource type / project)
│   └── state.go        `appState` (project ID, view kind) helpers
```

## Build & test

Task is the task runner (`Taskfile.yml`):

- `task run` — run the TUI locally (`go run ./cmd/tui`)
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
| `viewBuckets` | view_buckets.go | Storage buckets |
| `viewAccounts` | view_accounts.go | Service accounts |
| `viewLBs` / `viewLBsAll` | view_loadbalancers.go | Load balancers: Name, IP:Port, Region, Protocol, Scheme, Backends (+ Project in All) |

### Load balancer composition

GCP LBs have no single resource: a row = one forwarding rule (`ForwardingRule`, with `TargetKind`/`Target` parsed from the rule's target URL via `targetRef`). `Cache.ResolveLoadBalancer(projectID, region, name)` composes the tree from cache: rule → target proxy (`TargetProxy`) → URL map (`URLMap` with `PathRules`) → backend services (`BackendService` with `Backends` + `HealthChecks`); internal/classic rules target a backend service directly. The fetchers (`ListProjectForwardingRules`, `ListProjectTargetProxies`, `ListProjectURLMaps`, `ListProjectBackendServices`) each save to their own table (`forwarding_rules`, `target_proxies`, `url_maps`, `backend_services`); refresh (`refreshProjectLBs` in view_loadbalancers.go) fetches all four. `selectRow` shows the `lb` details overlay with the full chain; the Backends column shows the default backend service plus `(+N)` extra path-rule services.

Per-project views (`viewX`) use `appState.ProjectID`; the `viewXAll` variants cover all cached projects.

## UI architecture

- Single `tview.Table` (`a.info`) shared by all views. `populateInfoPanel()` (app.go) writes a bold fixed header row (`SetFixed(1, 0)`) + data rows; `applyFilter()` repaints with the active filter.
- Main layout (main.go): summary area / `tview.InputField` (`a.cmd`) / info table / status bar, wrapped in `tview.Pages` (`a.pages`).
- **Key handling** is two-layered:
  - Global: `a.inputCapture` (handlers.go) — `Ctrl-C` quits, `Esc` → `escapePressed()`, single-letter shortcuts (`shortcuts` table in handlers.go: `u` refresh, `y` copy row, `o` open console, `?` help, etc.).
  - Enter on a table row fires `a.info.SetSelectedFunc` → `app.selected()` → the current view's `selectRow`.
- **Command vs filter mode**: the bottom InputField is either a filter (`/` prefix or after pressing the filter shortcut, Enter → `filterEnter()`) or a command (`:`, Enter → `commandEnter()`). `commandEnter()` special-cases typed IP literals (`net.ParseIP`) by jumping to the IPs view.

## Overlays (floating windows)

Built on `tview.Pages` (stacked, hidden/shown pages), centered with spacer-Flexes:

- `help` page — `newHelpOverlay()` (handlers.go), toggled by `?` (`toggleHelp()`).
- `vm` page — VM details window, opened via VM views' `selectRow` → `showVMDetails()` (view_vms.go). `vmDetails()` builds the `[yellow]label: [-]value` color-tagged text.
- `ip` page — IP address details window, opened via IP views' `selectRow` → `showIPDetails()` (view_ips.go). `ipDetails()` formats all cached `IPAddress` fields.
- `lb` page — Load balancer details window, opened via LB views' `selectRow` → `showLBDetails()` (view_loadbalancers.go). `lbDetails()` renders the composed rule → proxy → URL map → backend services chain.
- All are dismissed by `Esc` — `escapePressed()` (handlers.go) removes the front page if it is `"vm"`, `"ip"`, or `"lb"`.

**Pattern to add a new floating window**: border+title on a `tview.TextView` with dynamic colors; height = line count + 2; wrap in `Flex(spacers) → FlexRow(spacers)`, width 80; `a.pages.RemovePage(name)` then `AddPage(name, overlay, true, true)`; add the page name to the Esc dismissal check.

## Data model & cache

Resource structs live in gcloud.go (e.g. `IPAddress` has `ProjectID, Name, Address, AddressType, Status, Region, Subnetwork, Network, UpdatedAt`). Fetchers follow `ListProject<Res>(projectID)` naming (e.g. `ListProjectIPAddresses` returns global + regional addresses). Short names for GCP resource URLs come from `resourceShortName`.

The cache (cache.go) mirrors each struct to a SQLite table (e.g. `ip_addresses`) with upsert-on-conflict. Reads: `ListX` (one project), `ListAllX` (all), `GetX` (exact lookup, may return multiple rows — e.g. `GetIPAddresses` one per subnetwork). Views always read from cache; `refresh` repopulates from the API in a goroutine and calls `QueueUpdateDraw` to repaint.

`console.go` builds GCP Console URLs per resource (`GCPVMURL`, `GCPAddressesURL`, `GCPVMLogsURL`, ...) — used by the `o` shortcut.

## Conventions

- No comments in code; keep it that way.
- New view fields are optional function fields on `viewDef` — views without a behavior leave them nil.
- Tests live next to the code (`*_test.go` in both packages); run `task test` before considering work done.
- Background work (API fetches) uses `a.tapp.QueueUpdateDraw` to touch UI state; errors during fetch go through `a.reportError` (status + log), never panic.