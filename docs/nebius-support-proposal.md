# Nebius AI Cloud support: investigation and proposal

Status: design decided, implementation in progress.

Goal: a second cloud in lazycloud with the same functionality and
resource mapping as GCP: TUI views, agent CLI (list/get), and the
shared cache. Investigation notes below; decisions at the end.

## How lazycloud couples to GCP today

Verified by direct read of the code before the registry refactor
(paths below are the pre-refactor `cmd/providers` layout, now
`pkg/providers`; see also
`~/.jcode/scratch/nebius-arch/architecture.md`):

- `cmd/providers/gcloud.go` (2150 lines): 32 exported fetchers, all
  GCP client libraries except two (`ListClusterWorkloads`,
  `ListClusterNodes` shell out to `gcloud` + `kubectl`). All 25+
  resource structs live in the same file with GCP semantics baked in
  (Zone/Region, `RoutingMode`, `roles/x` IAM grammar, GKE nodepool
  labels).
- `cmd/providers/cache.go` (3067 lines): one SQLite cache at
  `~/.config/lazycloud/cache.db`, 30 tables, every table keyed by
  `project_id` only. No cloud column or per-cloud namespace exists.
  `view_updates` and staleness tracking are cloud-neutral.
- `pkg/cli`: registry mechanics (dispatch, formats, table, exit
  codes) are provider-agnostic; `Fetchers` + `DefaultFetchers`
  hardcode 9 GCP fetchers; env var `LAZYCLOUD_PROJECT` scopes.
- `pkg/app` (TUI): view framework is generic (`viewDef`, `views`
  map, `setView`), but all 24 `view_*.go` files call
  `providers.List*` directly and every `consoleURL` points at
  console.cloud.google.com.
- Config: `config.yaml` has theme/shortcuts/autoupdate only. No
  cloud concept anywhere in config or state.

## Nebius facts (verified live against the nebius CLI, profile
`revolut-nebius`, tenant `tenant-e00a327xp4rybqad3h`)

Hierarchy: tenant -> project -> resources. Projects are
region-scoped (`spec.region`). Every list command takes
`--parent-id` and answers JSON:

- `nebius iam v2 tenant list --all` (tenant level)
- `nebius iam v2 project list --parent-id <tenant> --all`
- `nebius compute instance|disk list --parent-id <project> --all`
- `nebius mk8s cluster list --parent-id <project> --all`
- `nebius mk8s node-group list --parent-id <cluster> --all`
  (node groups hang off clusters, not projects)
- `nebius vpc network|subnet list --parent-id <project> --all`
- `nebius storage bucket list --parent-id <project> --all`
- `nebius iam service-account list --parent-id <project> --all`
- IAM: no member/role bindings like GCP. Access model:
  groups (tenant or project level) hold members
  (`iam group-membership list-members --parent-id <group>`) and
  access permits (`iam access-permit list --parent-id <group>`)
  grant a role on a resource id.

The nebius CLI also exposes `--profile`, `--format json|yaml|jsonpath`,
and `iam get-access-token` (bearer token, refreshed by the CLI).

## Resource mapping GCP -> Nebius

| lazycloud noun | GCP source | Nebius source | Notes |
|---|---|---|---|
| projects | resourcemanager | iam v2 project | region in spec |
| vms | compute instances | compute instance | platform/preset instead of machineType, no zone: subnet-scoped |
| disks | compute disks | compute disk | size_gibibytes, type |
| clusters | GKE | mk8s cluster | version, endpoints; node groups are a child resource |
| networks | VPC networks | vpc network | no routing mode; pools instead |
| subnetworks | subnets | vpc subnet | cidrs in status |
| buckets | GCS | storage bucket | S3-compatible, project-scoped |
| serviceaccounts | IAM SAs | iam service-account | id + name + active |
| iam | IAM bindings | groups + memberships + access permits | different model, mapping needed |

GCP-only TUI views with no direct Nebius equivalent stay GCP-only:
load balancers, PSC, cloud routers, VPNs, peerings, instance groups,
instance templates, reservations, images, snapshots, firewall rules,
DNS zones. Nebius equivalents exist for some (security groups for
firewalls) but are out of scope for the first PR.

## Decisions

1. Transport: gRPC through the Nebius Go SDK
   (`github.com/nebius/gosdk`), mirroring lazycloud's GCP fetchers
   which use native Google client libraries (29 of 31; only cluster
   workloads/nodes shell out, and only because they need kubeconfig
   access). Auth: `NEBIUS_IAM_TOKEN` env var override, otherwise
   the token comes from `nebius iam get-access-token` (still
   gRPC for data; the CLI is only the token source, same split
   gosdk itself documents).
2. Cloud selection: `--cloud gcp|nebius` global flag with
   `LAZYCLOUD_CLOUD` env default, gcp default.
3. Cache: per-cloud db files. `NewCache()` keeps
   `~/.config/lazycloud/cache.db` for GCP (no migration for
   existing users), Nebius gets `cache-nebius.db` via the same
   `NewCacheAt`.
4. Scope: Nebius project id occupies the same project slot;
   projects fetcher walks all tenants.
5. IAM: same noun, Nebius flavor renders group memberships and
   access permits.
6. PR split: PR #9 = Nebius provider + cache + agent CLI
   (this branch). TUI Nebius support is a follow-up PR #10
   (view set, console URLs, cloud picker), keeping one concern
   per PR.

## Authentication

Nebius auth is the same split the GCP side uses, different
mechanics. GCP: no auth code at all, the Google client
libraries resolve Application Default Credentials implicitly and
refresh tokens themselves (`gcloud auth application-default`
once, then invisible). Nebius: explicit two-step token source,
one token per fetch run.

1. `NEBIUS_IAM_TOKEN` env var, if set, wins.
2. Otherwise a one-shot `nebius iam get-access-token` (exec of the
   nebius CLI). On a federation profile (auth.nebius.com, Okta
   behind it) the CLI refreshes its cached token and returns in
   under a second, no browser; the browser dance happens once at
   `nebius profile create` and the CLI owns it from then on.
3. The token is passed to gosdk as a static bearer credential
   (`gosdk.WithCredentials(gosdk.IAMToken(token))`) and captured
   once per client. Each fetch run builds a fresh client
   (client-per-run, via `nebiusFetchers.open`), so a refresh
   cycle pulls a new token each time; inside a run the token
   never rotates.
4. Cache reads need no token; only fetching does.
5. Errors surface as the provider error with the pointer to
   `nebius profile create` or `NEBIUS_IAM_TOKEN` (exit code 3,
   infrastructure).

Why CLI-mediated instead of doing the OAuth ourselves: gosdk
v0.2.87 ships a native federation flow (browser callback server,
PKCE, authorize/token endpoints in `auth/federation/`) plus
rotating credential interfaces (`CustomTokener`), so lazycloud
could own the browser dance and refresh itself if the CLI
dependency ever needs to go. Deliberately not done now: the CLI
profile is the source of truth for federation config (endpoint,
federation id), keeping PR #9 small and mirroring how the GCP
side treats `gcloud` as its auth entry.

## Resource shapes verified live

Tenant/project/instance/disk/cluster/node-group/network/subnet/
bucket/service-account/group/membership/access-permit JSON was
sampled from the real tenant on 2026-11-10 with the nebius CLI;
 shapes are in the git history of this file and in the fetcher
 tests as fixtures.

## Implementation notes

- gosdk services are behind narrow local interfaces
  (`nebiusAPI`) so tests inject fakes; no test touches the
  network.
- Nebius project ids look like `project-e04...`; the projects
  fetcher lists tenants then projects per tenant.
- `iam` noun: members come from group memberships, roles from
  access permits, so one Nebius IAMBinding row = permit joined
  with the group members holding it.
- Nebius has no zones; VM Zone field carries the subnet-derived
  region so the CLI/TUI column set stays identical.

## Proposed implementation shape

(decisions applied above)

- `pkg/providers/nebius/nebius.go`: Nebius fetchers behind the same
  function signatures as the GCP ones, plus Nebius flavor structs
  where the GCP struct semantics do not fit (likely VM, Network,
  IAMBinding).
- `pkg/providers/cache.go`: chosen cache layout change.
- `pkg/cloud`: provider registry. One `Provider` entry per cloud
  (name, `Fetchers`, cache constructor); `pkg/cli` keeps only the
  `--cloud` flag grammar and resolves everything through the
  registry, so adding AWS later is one registration file with no
  edits elsewhere.
- `pkg/app`: cloud switch in run/app, same view set for Nebius
  where mapping exists, GCP-only views hidden when Nebius is active.
- Tests: fake Nebius CLI/server responses, 100% coverage of new
  code, no network.

Once decisions land, implementation begins on this branch
(`nebius-support`, stacked on `agent-cli-proposal` PR #8 head).

## Layout after the registry refactor

`pkg/providers` keeps the shared model structs and the cache.
Each cloud lives in its own adapter package: `pkg/providers/gcp`,
`pkg/providers/nebius` (package name carries the cloud, so
`nebius.ListVMs`, no `ListNebius` prefix). `pkg/cloud` is the
registry (one registration file per cloud), `pkg/cli` holds the
--cloud grammar and the noun registry, `pkg/app` imports the gcp
adapter for its GCP-only views until the TUI cloud switch (PR #10).
Adding a cloud: shared model untouched, one adapter package, one
registration file.

Schema caveats verified for the next cloud (read at cache.go HEAD):

- `vms.zone` and `disks.zone` sit in the PK with NOT NULL. A zoneless
  cloud already works through the empty string (Nebius proves it), but
  a cloud whose natural key is name + region + lifecycle state, not
  name + zone, would fight the PK.
- The networks table carries GCP vocabulary (`routing_mode`,
  `auto_create_subnets`, `private_ip_google_access`, `mtu`, `purpose`).
  A VPC-shaped cloud leaves these empty strings, harmless but
  GCP-shaped; a mapper zeroes them like Nebius does.
- `projects.project_number` is NOT NULL with no GCP meaning. Nebius
  carries the region there with a renamed column header; a cloud with
  neither number nor region needs a dummy value or a schema change.
- DDL runs on every open, all `CREATE TABLE IF NOT EXISTS`, and each
  cloud gets its own db file, so per-cloud tables or a wider schema
  cost nothing today. Migration relies on ad-hoc `ensureColumn`, fine
  while files stay per-cloud.

None of these block a third cloud; they mark the shared schema as
GCP-shaped optional, not cloud-neutral.

## Status (2026-11-10, PR #9)

Implemented: provider (`pkg/providers/nebius/nebius.go`), per-cloud
cache (`cache-nebius.db`), `--cloud` flag + `LAZYCLOUD_CLOUD` env
(registry in `pkg/cloud`, grammar in `pkg/cli/dispatch.go`), tests
with fake API injection
(`pkg/providers/nebius/nebius_test.go`, `pkg/cli/cloud_test.go`),
README CLI section. PR #9 scope ends here; TUI support remains
PR #10.

One deviation from the notes above: the shared VM struct was
enough for Nebius (no flavor structs needed), and Zone stays
empty rather than carrying a derived region, because the CLI
table tolerates the empty column and a fake region would be
wrong data. The IAM noun renders one row per (member, permit)
pair with `group:<name> resource:<id>` in the condition field.

Scope note: IAM groups are listed per project only
(`ListGroups` with the project as parent). Nebius also allows
tenant-level groups; a tenant-level listing changes the join
semantics (which permits apply to which scope) and is left for
the TUI follow-up rather than silently half-implemented here.
