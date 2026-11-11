# lazycloud agent CLI proposal

**Status: implemented (phase 1).** The `list`/`get`/`projects`/`resources` commands ship in `pkg/cli` with the eight flagship resources, all four output formats, `--project`/`LAZYCLOUD_PROJECT`, `--refresh`, and the exit-code contract described below. This document is the design rationale; run `lazycloud resources` for the live list and see the README's CLI section for usage.

lazycloud is a TUI today. This document proposes a companion CLI surface, `lazycloud list <resource>`, exposing the same resource browsing as structured JSON (or table) output, designed for scripts and AI agents (Claude Code, Codex, Jcode, Gemini CLI, cagent) instead of humans at a keyboard.

No TUI code is touched. The CLI reads the same cache and calls the same fetchers.

## Why

- **Agents cannot use the TUI.** `app.Run()` opens `/dev/tty`; inside an agent shell there is no TTY, so today the only agent path is raw `gcloud` with per-resource flag archaeology. The value of lazycloud, one consistent view over 30+ resource types with a local cache, is exactly what an agent needs but cannot reach.
- **The layer separation already exists.** `cmd/providers` has zero `tview`/`tcell` imports: 31 `List*` functions (gcloud.go, pure ADC auth, no flags) and 124 `Cache` methods already cover everything the TUI shows. The proposal is a thin dispatcher over that layer, not a rewrite.
- **JSON is already half-implemented.** The `y` copy key serializes any row's cached resource (handlers.go `rowCopyPayload`), and `:write` dumps whole views as JSON/YAML/CSV. The CLI generalizes what already exists.

## Non-goals

- No mutations (create/update/delete). Read-only, like the TUI. Mutations via agent CLI need scoping, dry-run and confirmation semantics, out of scope here.
- No TUI replacement, no flags mirroring gcloud 1:1. One noun, few flags, stable output.
- No new fetchers. Only what providers already expose.

## Proposed interface

```
lazycloud list <resource> [--project <id>] [--json|--yaml|--table|--ndjson] [--refresh]
lazycloud get <resource> <selector> [--project <id>] [--json|--yaml]
lazycloud projects            # shortcut: list projects
lazycloud resources           # list valid resource names
```

`--filter` (below) is phase 2.

Examples for an agent session:

```
$ lazycloud list vms --project my-prod --refresh
[{"project":"my-prod","name":"api-01","zone":"europe-west1-b",...}]

$ lazycloud get vm my-prod/europe-west1-b/api-01
{ "name": "api-01", "interfaces": [...], "labels": {...}, ... }

$ lazycloud list clusters --project my-prod --json | jq '.[] | select(.status=="RUNNING")'
```

## Design rules for agent usability

1. **Stable, lowercase snake_case JSON keys.** Today `VM` marshals as `ProjectID`/`Name`/... because the provider structs carry no json tags. The CLI adds `json:"..."` tags, giving scripts a contract that survives Go renames.
2. **JSON array by default, NDJSON behind `--ndjson`.** Arrays are friendlier to `jq` first steps; NDJSON is there for streaming consumers.
3. **Exit codes are a contract**: 0 ok, 1 usage (bad flag/resource), 2 not found (empty result is 0, not an error), 3 auth/quota/API error.
4. **stdout is data only, stderr is one human-readable diagnostic line.** A warning never lands in the output stream, so parsing stdout cannot break.
5. **`--project` flag and `LAZYCLOUD_PROJECT` env var.** Absent both: all projects (`ListAll*`), matching the TUI's `a` toggle. The `get` selector syntax `project/location/name` mirrors the cache keys.
6. **Cache-first with explicit `--refresh`.** Default reads the cache (instant, offline-safe); `--refresh` re-fetches from GCP before printing, the same operation as the TUI `u` key. Agents that need freshness pass `--refresh`; agents that need speed read a cache the user has already warmed.
7. **`--table` prints the exact same columns as the TUI view**, headers included, for humans debugging. `--json` is the default; `--yaml` reuses the `:write` marshaller.
8. **`resources` subcommand and per-resource `--help`** list valid nouns and flags, so an agent can discover the surface without reading the repo.
9. **No pagination flags.** Fetchers already pull full lists and the TUI renders every row; same behavior here, agents filter client-side or with `--filter`.

## Implementation analysis

### Where the code goes

```
main.go            dispatch: no args -> app.Run(); first arg a subcommand -> cli.Run
pkg/cli/
├── run.go         argument parsing, exit codes, output format selection
├── resources.go   resource registry: noun -> {fetcher, cacheReader, tableColumns}
└── table.go       --table renderer
```

The registry is the only place that maps nouns to functions. Each entry is realistically 10-20 lines (fetch + cache readers + headers), added incrementally starting with the top TUI views: projects, vms, disks, clusters, networks, buckets, service accounts, IAM.

### Registry entry sketch

```go
type resourceDef struct {
    name        string                        // "vms"
    aliases     []string                      // ["vm"]
    all         func() ([]any, error)         // cache ListAll* -> []any
    project     func(string) ([]any, error)   // cache List*(projectID)
    refresh     func(string) ([]any, error)   // fetch + save for one project
    headers     []string                     // TUI columns for --table
    row         func(any) []string           // one --table row
}
```

The awkward part: cache methods return concrete types (`[]providers.VM`) and marshaling happens at the edges. Options considered:

- **A. `[]any` in the registry** (sketch above). Simple, some `any` plumbing at registration; `json.Marshal` plus the tags makes it stable. Chosen.
- **B. Generics.** Cleaner signatures, but each resource still needs its own helpers and the registry map gets awkward. Rejected.
- **C. Reuse `viewDef` from pkg/app.** Tempting: headers, `load` closures and `fetch` closures already exist per view. But `viewDef` methods take `*app` (TUI state: statusText, QueueUpdateDraw, appState.ProjectID), so reuse needs a refactor lifting `load`/`fetch` out of `*app` first. Right long-term, not needed for a read-only CLI. Deferred, see phase 3.

### main.go dispatch

```go
func main() {
    if len(os.Args) > 1 && isSubcommand(os.Args[1]) {
        os.Exit(cli.Run(os.Args[1:], version))
    }
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Println(version)
        return
    }
    // no args: TUI, unchanged
    if err := app.Run(); err != nil { ... }
}
```

TUI behavior is unchanged; `lazycloud` with no args still opens the TUI. Existing users see nothing new except `--version` and the subcommands.

### Output: json tags (the one invasive part)

The provider structs need `json:"snake_case"` tags. This also changes `y`-copy and `:write` JSON output (PascalCase to snake_case). The TUI itself renders rows, not raw JSON, so nothing visual changes; the only behavioral change is the copy/write payload key casing. Alternatives considered and rejected:

- A separate CLI DTO layer: duplicates the provider structs and drifts.
- PascalCase in CLI output: agents expect snake_case.

Recommendation: add the tags, accept the `y`/`:write` casing change, note it in the changelog. The tags are mechanical to add.

### Filter

`--filter "status=RUNNING,zone~west1"`: comma-separated ANDs, `=` exact, `~` substring, applied over the JSON view of each object (lowercased key lookup, case-insensitive values). Implemented once in pkg/cli, works for every resource. Deferred to phase 2 so phase 1 lands as pure list/get with formats.

### Auth and dependencies

Fetchers use ADC implicitly (resourcemanager/compute REST clients created with no options). Agent environments need `gcloud auth application-default login` or workload identity, same requirement as the TUI. Workloads and nodes still require `gcloud` + `kubectl` on PATH (exec out, temp KUBECONFIG), unchanged.

### Cache sharing with the TUI

Same `~/.config/lazycloud/cache.db` (modernc.org/sqlite, no cgo). Concurrent TUI + CLI is safe for reads; `--refresh` writes take SQLite locks per statement, the same as the TUI's own writes. A cache-status subcommand (per-view `updated_at` from the `view_updates` table) is a phase 2 nice-to-have: it lets an agent decide cache-first vs `--refresh` without guessing.

### Test surface

- Registry unit tests: every entry resolves fetcher/headers and round-trips JSON marshal.
- Exit-code tests: unknown resource -> 1, empty result -> 0 with `[]` on stdout.
- Golden JSON tests per resource fixture: output shape locked so the agent contract cannot drift silently.
- `--table` headers match the TUI viewDef headers for shared resources (until the phase 3 refactor unifies them).

## Phasing

1. **Phase 1:** `list`/`get`/`projects`/`resources`, `--json` default, `--table`, `--project`, implicit all-projects, `--refresh`, `--yaml`, `--ndjson`, json tags, exit codes, tests. Registry seeded with 7-8 flagship resources (projects, vms, disks, clusters, networks, buckets, service accounts, IAM).
2. **Phase 2:** all remaining resources (LBs, DNS zones/records, PSC, VPNs, cloud routers, images, instance templates, snapshots, reservations, instance groups, peerings, routes, firewalls, subnets), `--filter`, `resources --verbose` (per-resource flags), cache-status subcommand.
3. **Phase 3:** viewDef refactor lifting `load`/`fetch` out of `*app` so TUI and CLI share one registry (kills the duplicated headers), opening the door to an MCP server wrapper over the same registry if agents with MCP support want it.

## Alternative surfaces considered

- **MCP server**: best ergonomics inside MCP-capable agents, but a second protocol to maintain. A CLI printing JSON is usable by every agent today (subprocess + stdout); the registry would be identical if MCP is added later.
- **gRPC/HTTP daemon**: overkill for a local read-only tool.
- **Extending `:write`**: TUI-bound, not agent-reachable.

## Open questions for the maintainer

1. Appetite for the `y`/`:write` JSON key casing change (snake_case tags) in a minor release with a changelog note?
2. Should `get` exist in phase 1, or is `list --filter` enough? (Recommendation: keep `get`; the `project/location/name` selector is the agent-friendly identity form.)

## Work estimate

Phase 1: registry + dispatch + tags + tests, roughly 600-900 lines including tests, one focused PR, no new dependencies.
