package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
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
	// Refresh re-fetches from GCP before printing.
	Refresh bool
	// Format is one of json, yaml, ndjson, table.
	Format string
}

// deps are the side-effectful inputs of Execute. defaultDeps wires the
// production cache, fetchers, and environment; tests inject fakes, so
// every path runs without touching the real cache or GCP.
type deps struct {
	openCache  func() (*providers.Cache, error)
	fetchers   Fetchers
	envProject func() string
	// saveViewUpdate records freshness for the TUI; the default is the
	// opened cache's method, tests inject a fake.
	saveViewUpdate func(cache *providers.Cache, view, projectID string) error
}

func defaultDeps() deps {
	return deps{
		openCache:  providers.NewCache,
		fetchers:   DefaultFetchers(),
		envProject: func() string { return os.Getenv("LAZYCLOUD_PROJECT") },
		saveViewUpdate: func(cache *providers.Cache, view, projectID string) error {
			return cache.SaveViewUpdate(view, projectID)
		},
	}
}

// Execute dispatches one CLI invocation (args[0] is the program name,
// matching os.Args) and returns the process exit code. Data goes to
// stdout, one diagnostic line to stderr on failure. NotHandled means
// the invocation is not a CLI command and main starts the TUI.
//
// Grammar:
//
//	lazycloud resources
//	lazycloud projects [flags]
//	lazycloud list <resource> [--project ID] [--refresh] [--json|--yaml|--ndjson|--table]
//	lazycloud get <resource> <selector> [flags]
func Execute(args []string, stdout, stderr io.Writer) int {
	return ExecuteDeps(args, stdout, stderr, defaultDeps())
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
	if err := renderItems(items, opts.Format, def, stdout); err != nil {
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
