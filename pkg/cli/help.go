package cli

import (
	"fmt"
	"strings"
)

// helpText renders the --help output: the whole CLI surface on one
// screen, so agents discover the subcommands, flags, resources and
// selectors without reading the repo. resourceNouns stays the single
// source of truth for the resource list.
func helpText(version string) string {
	var b strings.Builder
	if version != "" {
		fmt.Fprintf(&b, "lazycloud %s — browse GCP resources in the terminal\n", version)
	} else {
		b.WriteString("lazycloud — browse GCP resources in the terminal\n")
	}
	b.WriteString(`
Usage:
  lazycloud                                 interactive TUI (no arguments)
  lazycloud --version                       print the version
  lazycloud --help                          show this help
  lazycloud resources                       list the valid resource names
  lazycloud projects [flags]                list projects
  lazycloud list <resource> [flags]         list a resource from the cache
  lazycloud get <resource> <selector> [flags]
                                            show one resource by selector

Flags:
  --project <id>     scope to one project (default: LAZYCLOUD_PROJECT, else all projects)
  --refresh          re-fetch from GCP before printing (list only)
  --json             JSON array, the default
  --ndjson           one JSON object per line
  --yaml             YAML
  --table            table with the TUI view's columns

Resources: `)
	b.WriteString(strings.Join(resourceNouns, ", "))
	b.WriteString(`
Aliases: the singular of every noun, plus sa for serviceaccounts.

Selectors: project (projects), project/name or project/zone/name (vms, disks),
project/name or project/region/name (networks: VPCs, subnets),
project/name (clusters, buckets), project/email (serviceaccounts),
project/member/role (iam; the role keeps its slashes).

stdout is data only, diagnostics go to stderr.
Exit codes: 0 ok, 1 usage, 2 not found, 3 infrastructure error.`)
	return b.String()
}