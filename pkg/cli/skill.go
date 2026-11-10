package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Skill command flags and paths. The skill installs under
// <base>/lazycloud/SKILL.md, the layout agent runtimes expect for
// user-installed skills.
const (
	skillName         = "lazycloud"
	skillFileName     = "SKILL.md"
	envSkillsDirVar   = "LAZYCLOUD_SKILLS_DIR"
	defaultSkillsRoot = ".agents/skills"
)

// runSkill handles "lazycloud skill install [--path <dir>]": it
// resolves the skills base dir (--path, then LAZYCLOUD_SKILLS_DIR,
// then ~/.agents/skills), writes the skill, and prints the installed
// path on stdout. Re-running overwrites, so an update is the same
// command again.
func runSkill(args []string, stdout, stderr io.Writer, d deps) int {
	if len(args) < 3 || args[2] != "install" {
		return usageErr(stderr, "usage: %s skill install [--path <dir>]", args[0])
	}
	dir := ""
	for i := 3; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--path":
			if i+1 >= len(args) {
				return usageErr(stderr, "--path requires a value")
			}
			i++
			dir = args[i]
		case strings.HasPrefix(arg, "--path="):
			dir = strings.TrimPrefix(arg, "--path=")
		case strings.HasPrefix(arg, "--"):
			return usageErr(stderr, "unknown flag %q", arg)
		default:
			return usageErr(stderr, "unexpected argument %q", arg)
		}
	}
	// --path wins without consulting the env, so a bad home or env
	// cannot block an explicit target.
	base := dir
	if base == "" {
		var err error
		base, err = d.skillsBaseDir()
		if err != nil {
			return failErr(stderr, "resolving skills dir: %v", err)
		}
	}
	target, err := d.installSkill(base, skillName, skillContent())
	if err != nil {
		return failErr(stderr, "installing skill: %v", err)
	}
	fmt.Fprintln(stdout, target)
	return exitOK
}

// defaultSkillsBaseDir resolves the install base: the env override
// when set, otherwise ~/.agents/skills.
func defaultSkillsBaseDir() (string, error) {
	if dir := os.Getenv(envSkillsDirVar); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, filepath.FromSlash(defaultSkillsRoot)), nil
}

// installSkillFiles writes the skill content under base/name and
// returns the file path. Directories are created as needed; an
// existing SKILL.md is overwritten so installs stay idempotent.
func installSkillFiles(base, name, content string) (string, error) {
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, skillFileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// skillContent returns the installed SKILL.md: the CLI contract an
// agent needs (grammar, selectors, exit codes) plus investigation
// recipes. It documents only what the binary doing the installing
// supports, so it stays true per stack.
func skillContent() string {
	return `---
name: lazycloud
description: Investigate cloud resources through the lazycloud CLI. Use when asked to find, list, or analyse GCP or Nebius resources: VMs, disks, clusters, networks, buckets, service accounts, IAM.
---

# lazycloud

Cloud TUI + agent CLI over one SQLite cache per cloud. GCP: ~/.config/lazycloud/cache.db. Nebius: cache-nebius.db. CLI = data, JSON, fast. TUI = humans.

## Commands

- resources: the valid nouns
- projects: shortcut for "list projects"
- list <noun> [flags]: many rows
- get <noun> <selector>: one row

Nouns: projects, vms, disks, clusters, networks, buckets, serviceaccounts, iam.
Aliases: vm, disk, cluster, network, bucket, sa, serviceaccount, project.

Selectors for get:
- projects: project
- vms, disks: project/zone/name
- clusters, buckets, VPCs: project/name
- subnets: project/region/name
- service accounts: project/email
- iam: project/member/role, the role keeps its slashes (roles/viewer)

The networks noun lists VPCs and subnets together; the kind column (vpc, subnet) tells them apart.

## Flags

- --cloud gcp|nebius: pick the provider. gcp is the default. LAZYCLOUD_CLOUD fills the flag when absent. Each cloud keeps its own cache: switching never mixes data.
- --project <id>: scope one project. Absent, and no LAZYCLOUD_PROJECT env: all projects.
- --refresh: fetch from the cloud before printing. List only. When a multi-project refresh partially fails, refreshed data still prints on stdout, failures land on stderr, exit code is 3.
- --json (default, compact array), --ndjson (one object per line), --yaml, --table (the TUI columns).

## Output contract

- snake_case keys. stdout = data only. [] is a valid empty result. Warnings and errors go to stderr.
- Exit codes: 0 ok, 1 usage, 2 not found, 3 infra (cache, auth, quota, API).

## Nebius setup

- Token: NEBIUS_IAM_TOKEN env, or the nebius CLI (nebius profile create, then nebius iam get-access-token runs behind the scenes).
- Project ids are Nebius iam project ids, not GCP-style strings.
- The projects table reuses the Project Number column for the Nebius region.
- VMs have no zone: the zone field stays empty. Region lives on the project row.
- Disks: size_gb comes from status.size_bytes; users lists attached instances.
- Networks: subnets carry ip_range, the CIDR pools comma-joined.
- IAM: rows come from groups with access permits; condition carries group:<name> resource:<id>. Tenant-level groups are out of scope; a project scope lists that project's groups.

## Investigation recipes

- Inventory: "lazycloud projects --refresh" first, then per-project lists.
- By type: "lazycloud list <noun> --project <id>".
- By location: zone in vm and disk selectors and the zone column; region in the subnet selector; clusters and buckets carry location. Filter JSON client-side for exact matches.
- One known item: get with its selector beats list plus filtering.
- Freshness: rows carry updated_at. Stale or unsure: add --refresh.
- Unused disks: list disks, users field empty = deletion candidate.
- Oversized VMs: machine_type per vm, compare against observed load.
- IAM audit: list iam --project <id>, the member prefix is the type (user, serviceAccount, group).
- Network layout: list networks --project <id>, vpc and subnet rows, ip_range column.
- Exit 2: wrong name, fix the selector. Exit 3: infra, check auth (gcloud or nebius), then retry.

## Judgement

- Cache reads are instant and possibly stale. --refresh costs API quota; use it when truth matters.
- Refresh needs the cloud CLI installed and authed (gcloud for GCP, nebius for Nebius); cache reads need nothing.
- Prefer JSON for parsing, table for eyeballing.
- One cloud per invocation: repeat the command with the other --cloud instead of assuming one view covers both.
`
}
