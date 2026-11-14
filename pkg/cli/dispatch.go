package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	cloudProviders "github.com/elferherrera/lazycloud/pkg/cloud"
	"github.com/elferherrera/lazycloud/pkg/providers"
)

// NotHandled is returned by Execute when the invocation is not a CLI
// command; main then starts the TUI. It is a sentinel, never a
// process exit code.
const NotHandled = -1

// Exit codes are the CLI's contract with agents: 0 success (an empty
// result included), 1 usage, 2 not found, 3 infrastructure.
const (
	exitOK       = 0
	exitUsage    = 1
	exitNotFound = 2
	exitError    = 3
)

// errNoProjectsCached tells the agent the cache holds no projects to
// refresh against; the fix is refreshing the project list first.
var errNoProjectsCached = errors.New("no projects in cache, run: lazycloud projects --refresh")

// Options carries the parsed flags of one invocation. The zero value
// plus the environment default lists all projects in json.
type Options struct {
	// Project scopes list and refresh to one project; empty means all
	// projects, mirroring the TUI's a toggle.
	Project string
	// Refresh re-fetches from the cloud before printing.
	Refresh bool
	// Format is one of json, yaml, ndjson, table.
	Format string
}

// deps are the side-effectful inputs of Execute. defaultDeps wires the
// production cache, fetchers, and environment; tests inject fakes, so
// every path runs without touching the real cache or any cloud.
type deps struct {
	openCache  func() (*providers.Cache, error)
	fetchers   Fetchers
	envProject func() string
	// tableHeaders overrides the --table columns per resource noun;
	// nil or a missing noun means the standard columns.
	tableHeaders map[string][]string
	// saveViewUpdate records freshness for the TUI; the default is the
	// opened cache's method, tests inject a fake.
	saveViewUpdate func(cache *providers.Cache, view, projectID string) error
	// skillsBaseDir resolves the skill install base dir; the default
	// reads LAZYCLOUD_SKILLS_DIR then ~/.agents/skills, tests inject
	// a fixed dir so no test touches the real home.
	skillsBaseDir func() (string, error)
	// installSkill writes one skill under a base dir; the default
	// touches the filesystem, tests inject a fake for error paths.
	installSkill func(base, name, content string) (string, error)
}

func defaultDeps() deps {
	p, _ := cloudProviders.Get(cloudProviders.GCP)
	return deps{
		openCache:  providers.NewCache,
		fetchers:   p.Fetchers,
		envProject: func() string { return os.Getenv("LAZYCLOUD_PROJECT") },
		saveViewUpdate: func(cache *providers.Cache, view, projectID string) error {
			return cache.SaveViewUpdate(view, projectID)
		},
		skillsBaseDir: defaultSkillsBaseDir,
		installSkill:  installSkillFiles,
	}
}

// defaultDepsForCloud resolves a cloud name through the pkg/cloud
// registry: cache and fetchers come from the same Provider entry, so
// an invocation never mixes one cloud's cache with another's
// fetchers. The valid names and the default live in the registry,
// not here.
func defaultDepsForCloud(cloudName string) (deps, error) {
	p, err := cloudProviders.Get(cloudName)
	if err != nil {
		return deps{}, err
	}
	d := defaultDeps()
	d.fetchers = p.Fetchers
	d.tableHeaders = p.TableHeaders
	if p.NewCache != nil {
		d.openCache = p.NewCache
	}
	return d, nil
}

// Execute dispatches one CLI invocation (args[0] is the program name,
// matching os.Args) and returns the process exit code. Data goes to
// stdout, one diagnostic line to stderr on failure. NotHandled means
// the invocation is not a CLI command and main starts the TUI.
// LAZYCLOUD_CLOUD and --cloud are only consulted for CLI commands,
// so a bad env value never blocks the TUI.
//
// Grammar:
//
//	lazycloud resources
//	lazycloud projects [flags]
//	lazycloud list <resource> [--cloud <name>] [--project ID] [--refresh] [--json|--yaml|--ndjson|--table]
//	lazycloud get <resource> <selector> [flags]
//
// LAZYCLOUD_CLOUD fills --cloud when the flag is absent; the default
// cloud and the valid names come from the pkg/cloud registry.
func Execute(args []string, stdout, stderr io.Writer) int {
	if isCLICommand(args) {
		envCloud := cloudProviders.FromEnv()
		cloudName, rest, err := splitCloudFlag(args, envCloud)
		if err != nil {
			fmt.Fprintf(stderr, "usage error: %v\n", err)
			return exitUsage
		}
		d, err := defaultDepsForCloud(cloudName)
		if err != nil {
			// An unknown cloud matching the env value came from the
			// env (the flag is stripped from rest by now), so the
			// error names LAZYCLOUD_CLOUD; anything else reads as a
			// bad flag value.
			if cloudName == envCloud {
				var uc *cloudProviders.ErrUnknownCloud
				if errors.As(err, &uc) {
					uc.FromEnv = true
				}
			}
			fmt.Fprintf(stderr, "usage error: %v\n", err)
			return exitUsage
		}
		return ExecuteDeps(rest, stdout, stderr, d)
	}
	return NotHandled
}

// isCLICommand reports whether args name one of the subcommands;
// --cloud may sit before the subcommand, so the scan skips flag
// pairs. Anything else falls through to the TUI.
func isCLICommand(args []string) bool {
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "resources", "projects", "list", "get", "skill":
			return true
		case "--cloud":
			i++
		}
	}
	return false
}

// splitCloudFlag extracts the --cloud value and returns the args with
// the flag pair removed. The env default applies when the flag is
// absent; a flag wins over the env. Validity is decided by the
// registry, so this layer only checks that a value was provided.
func splitCloudFlag(args []string, envCloud string) (string, []string, error) {
	cloudName := envCloud
	rest := make([]string, 0, len(args))
	flagSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--cloud":
			if i+1 >= len(args) {
				return "", nil, errors.New("--cloud requires a value")
			}
			i++
			cloudName = args[i]
			flagSet = true
		case strings.HasPrefix(arg, "--cloud="):
			cloudName = strings.TrimPrefix(arg, "--cloud=")
			flagSet = true
		default:
			rest = append(rest, arg)
		}
	}
	if flagSet && cloudName == "" {
		return "", nil, fmt.Errorf("--cloud requires a value, valid: %s", strings.Join(cloudProviders.Names(), " "))
	}
	return cloudName, rest, nil
}

// ExecuteDeps runs Execute with injected dependencies, the seam for
// tests and for callers wiring a non-default cache.
func ExecuteDeps(args []string, stdout, stderr io.Writer, d deps) int {
	if len(args) < 2 {
		return NotHandled
	}
	switch args[1] {
	case "resources":
		fmt.Fprintln(stdout, strings.Join(ResourceNames(), " "))
		return exitOK
	case "projects":
		listArgs := append([]string{args[0], "list", "projects"}, args[2:]...)
		return ExecuteDeps(listArgs, stdout, stderr, d)
	case "list", "get":
		return runCommand(args, stdout, stderr, d)
	case "skill":
		return runSkill(args, stdout, stderr, d)
	}
	return NotHandled
}

// runCommand parses the flags after the resource noun and runs the
// command against an opened cache.
func runCommand(args []string, stdout, stderr io.Writer, d deps) int {
	cmd := args[1]
	rest := args[2:]
	if len(rest) == 0 {
		return usageErr(stderr, "missing resource, valid: %s", strings.Join(ResourceNames(), " "))
	}
	noun := ResolveAlias(rest[0])
	def, ok := defaultRegistry(d.fetchers)[noun]
	if !ok {
		return usageErr(stderr, "unknown resource %q, valid: %s", rest[0], strings.Join(ResourceNames(), " "))
	}
	opts, sel, err := parseFlags(cmd, rest[1:], d.envProject)
	if h, ok := d.tableHeaders[noun]; ok {
		def.headers = h
		if len(def.headersAll) > 0 {
			def.headersAll = withProjectColumn(h)
		}
	}
	if err != nil {
		return usageErr(stderr, "%s", err)
	}
	cache, err := d.openCache()
	if err != nil {
		return failErr(stderr, "opening cache: %v", err)
	}
	defer cache.Close()
	if cmd == "list" {
		return runList(noun, def, opts, cache, stdout, stderr, d.saveViewUpdate)
	}
	return runGet(noun, def, sel, opts, cache, stdout, stderr)
}

// parseFlags reads the flags after the resource noun. Non-flag
// arguments are get selector parts, split on /; list takes none.
// LAZYCLOUD_PROJECT fills --project when the flag is absent.
func parseFlags(cmd string, args []string, envProject func() string) (Options, []string, error) {
	opts := Options{Project: envProject(), Format: formatJSON}
	var sel []string
	seenFormat := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--project":
			if i+1 >= len(args) {
				return opts, nil, errors.New("--project requires a value")
			}
			i++
			opts.Project = args[i]
		case strings.HasPrefix(arg, "--project="):
			opts.Project = strings.TrimPrefix(arg, "--project=")
		case arg == "--refresh":
			opts.Refresh = true
		case formatFlag(arg) != "":
			f := formatFlag(arg)
			if seenFormat && opts.Format != f {
				return opts, nil, errors.New("conflicting format flags")
			}
			seenFormat = true
			opts.Format = f
		case strings.HasPrefix(arg, "--"):
			return opts, nil, fmt.Errorf("unknown flag %q", arg)
		case cmd == "list":
			return opts, nil, fmt.Errorf("unexpected argument %q", arg)
		default:
			sel = append(sel, strings.Split(arg, "/")...)
		}
	}
	if cmd == "get" && len(sel) == 0 {
		return opts, nil, errors.New("missing selector")
	}
	if cmd == "get" && opts.Refresh {
		return opts, nil, errors.New("--refresh is only valid for list")
	}
	return opts, sel, nil
}

// formatFlag maps a format flag to its format, or "" when the
// argument is not a format flag.
func formatFlag(arg string) string {
	switch arg {
	case "--json":
		return formatJSON
	case "--yaml":
		return formatYAML
	case "--ndjson":
		return formatNDJSON
	case "--table":
		return formatTable
	}
	return ""
}

// runList refreshes when asked, reads the cache, and prints.
func runList(noun string, def resourceDef, opts Options, cache *providers.Cache, stdout, stderr io.Writer, saveViewUpdate func(*providers.Cache, string, string) error) int {
	if opts.Project != "" && def.project == nil {
		return usageErr(stderr, "%s is global, --project does not apply", noun)
	}
	if opts.Refresh {
		targets, err := refreshTargets(def, opts.Project, cache)
		if err != nil {
			if errors.Is(err, errNoProjectsCached) {
				return failNotFound(stderr, "%s", err)
			}
			return failErr(stderr, "refresh: %v", err)
		}
		var errs []error
		for _, target := range targets {
			if err := def.refresh(cache, target); err != nil {
				if target == "" {
					errs = append(errs, err)
				} else {
					errs = append(errs, fmt.Errorf("%s: %w", target, err))
				}
			}
		}
		if err := errors.Join(errs...); err != nil {
			return failErr(stderr, "refresh: %v", err)
		}
		recordRefreshed(def, targets, cache, stderr, saveViewUpdate)
	}
	items, err := listItems(def, opts.Project, cache)
	if err != nil {
		return failErr(stderr, "reading cache: %v", err)
	}
	// Unscoped lists print the TUI's all-projects columns, Project
	// column included.
	listDef := def
	if opts.Project == "" && def.project != nil {
		listDef = resourceDef{
			headers: def.headersAll,
			row:     def.rowAll,
		}
	}
	if err := renderItems(items, opts.Format, listDef, stdout); err != nil {
		return failErr(stderr, "rendering: %v", err)
	}
	return exitOK
}

// refreshTargets lists the project scopes a refresh must cover: the
// one given project, or every cached project. Global resources
// (projects) have a single unscoped target.
func refreshTargets(def resourceDef, projectID string, cache *providers.Cache) ([]string, error) {
	if projectID != "" {
		return []string{projectID}, nil
	}
	if def.project == nil {
		return []string{""}, nil
	}
	cached, err := cache.ListProjects()
	if err != nil {
		return nil, err
	}
	if len(cached) == 0 {
		return nil, errNoProjectsCached
	}
	targets := make([]string, len(cached))
	for i, p := range cached {
		targets[i] = p.ProjectID
	}
	return targets, nil
}

// recordRefreshed marks the refreshed views fresh in the cache so the
// TUI's staleness display stays honest across both surfaces. A
// recording failure only warns: the data itself is saved.
func recordRefreshed(def resourceDef, targets []string, cache *providers.Cache, stderr io.Writer, saveViewUpdate func(*providers.Cache, string, string) error) {
	for _, target := range targets {
		if err := saveViewUpdate(cache, def.viewKind, target); err != nil {
			fmt.Fprintf(stderr, "warning: recording view update: %v\n", err)
			return
		}
	}
}

// listItems reads the resource from the cache, scoped to one project
// when both the scope and the resource support it. The projects
// resource is global and ignores the scope.
func listItems(def resourceDef, projectID string, cache *providers.Cache) ([]any, error) {
	if projectID != "" && def.project != nil {
		return def.project(cache, projectID)
	}
	return def.all(cache)
}

// runGet reads one resource by selector and prints it.
func runGet(noun string, def resourceDef, sel []string, opts Options, cache *providers.Cache, stdout, stderr io.Writer) int {
	item, err := def.get(cache, sel)
	if err != nil {
		if errors.Is(err, errBadSelector) {
			return usageErr(stderr, "bad selector %q for %s, expected %s", strings.Join(sel, "/"), noun, def.selectorHint)
		}
		return failErr(stderr, "get: %v", err)
	}
	if item == nil {
		return failNotFound(stderr, "%s %s not found", noun, strings.Join(sel, "/"))
	}
	if err := renderSingle(item, opts.Format, def, stdout); err != nil {
		return failErr(stderr, "rendering: %v", err)
	}
	return exitOK
}

func usageErr(stderr io.Writer, format string, args ...any) int {
	fmt.Fprintf(stderr, "usage error: "+format+"\n", args...)
	return exitUsage
}

func failNotFound(stderr io.Writer, format string, args ...any) int {
	fmt.Fprintf(stderr, "not found: "+format+"\n", args...)
	return exitNotFound
}

func failErr(stderr io.Writer, format string, args ...any) int {
	fmt.Fprintf(stderr, "error: "+format+"\n", args...)
	return exitError
}
