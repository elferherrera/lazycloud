package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cloudreg "github.com/elferherrera/lazycloud/pkg/cloud"
)

func TestSplitCloudFlagDefaults(t *testing.T) {
	cloud, rest, err := splitCloudFlag([]string{"lazycloud", "list", "vms"}, "")
	if err != nil || cloud != "" {
		t.Fatalf("splitCloudFlag default = %q, %v", cloud, err)
	}
	if len(rest) != 3 || rest[1] != "list" {
		t.Fatalf("rest = %v", rest)
	}
}

func TestSplitCloudFlagGCPExplicit(t *testing.T) {
	cloud, rest, err := splitCloudFlag([]string{"lazycloud", "--cloud", "gcp", "list", "vms"}, "")
	if err != nil || cloud != cloudreg.GCP {
		t.Fatalf("splitCloudFlag gcp = %q, %v", cloud, err)
	}
	if len(rest) != 3 || rest[0] != "lazycloud" || rest[1] != "list" || rest[2] != "vms" {
		t.Fatalf("rest = %v", rest)
	}
}

func TestSplitCloudFlagNebiusEqualsForm(t *testing.T) {
	cloud, rest, err := splitCloudFlag([]string{"lazycloud", "list", "vms", "--cloud=nebius"}, "")
	if err != nil || cloud != cloudreg.Nebius {
		t.Fatalf("splitCloudFlag nebius = %q, %v", cloud, err)
	}
	if len(rest) != 3 || rest[2] != "vms" {
		t.Fatalf("rest = %v", rest)
	}
}

func TestSplitCloudFlagEnvDefault(t *testing.T) {
	cloud, _, err := splitCloudFlag([]string{"lazycloud", "list", "vms"}, "nebius")
	if err != nil || cloud != cloudreg.Nebius {
		t.Fatalf("env default = %q, %v", cloud, err)
	}
}

func TestSplitCloudFlagFlagWinsOverEnv(t *testing.T) {
	cloud, _, err := splitCloudFlag([]string{"lazycloud", "--cloud", "gcp", "list", "vms"}, "nebius")
	if err != nil || cloud != cloudreg.GCP {
		t.Fatalf("flag should win = %q, %v", cloud, err)
	}
}

func TestSplitCloudFlagInvalidFlagValue(t *testing.T) {
	_, rest, err := splitCloudFlag([]string{"lazycloud", "--cloud", "aws", "list", "vms"}, "")
	if err != nil {
		t.Fatalf("splitCloudFlag must defer validity to the registry: %v", err)
	}
	if len(rest) != 3 || rest[1] != "list" {
		t.Fatalf("rest = %v", rest)
	}
}

func TestSplitCloudFlagInvalidEnvValue(t *testing.T) {
	cloudName, _, err := splitCloudFlag([]string{"lazycloud", "list", "vms"}, "azure")
	if err != nil {
		t.Fatalf("splitCloudFlag must defer validity to the registry: %v", err)
	}
	if cloudName != "azure" {
		t.Fatalf("env default = %q, want azure passed through", cloudName)
	}
}

func TestSplitCloudFlagMissingValue(t *testing.T) {
	_, _, err := splitCloudFlag([]string{"lazycloud", "list", "vms", "--cloud"}, "")
	if err == nil || !strings.Contains(err.Error(), "requires a value") {
		t.Fatalf("missing value error = %v", err)
	}
}

func TestSplitCloudFlagDoubleFlagWinsLast(t *testing.T) {
	cloud, rest, err := splitCloudFlag([]string{"lazycloud", "--cloud", "gcp", "--cloud", "nebius", "list"}, "")
	if err != nil || cloud != cloudreg.Nebius {
		t.Fatalf("last flag should win = %q, %v", cloud, err)
	}
	if len(rest) != 2 {
		t.Fatalf("rest = %v", rest)
	}
}

// TestExecuteInvalidCloudFlag covers the production entry point: a
// bad --cloud value fails before any network wiring happens, so this
// runs against the real Execute.
func TestExecuteInvalidCloudFlag(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Execute([]string{"lazycloud", "--cloud", "aws", "list", "vms"}, stdout, stderr)
	if code != exitUsage {
		t.Fatalf("exit = %d, want usage failure", code)
	}
	if !strings.Contains(stderr.String(), "aws") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// TestExecuteInvalidCloudEnv covers the env var failing through the
// production entry: the registry rejects the value and the error
// names LAZYCLOUD_CLOUD, with restore.
func TestExecuteInvalidCloudEnv(t *testing.T) {
	old, had := os.LookupEnv("LAZYCLOUD_CLOUD")
	t.Setenv("LAZYCLOUD_CLOUD", "azure")
	defer func() {
		if had {
			os.Setenv("LAZYCLOUD_CLOUD", old)
		} else {
			os.Unsetenv("LAZYCLOUD_CLOUD")
		}
	}()
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Execute([]string{"lazycloud", "list", "vms"}, stdout, stderr)
	if code != exitUsage {
		t.Fatalf("exit = %d, want usage failure", code)
	}
	if !strings.Contains(stderr.String(), "LAZYCLOUD_CLOUD") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// TestExecuteInvalidCloudEnvIgnoredByTUI covers the contract that a
// bad LAZYCLOUD_CLOUD never blocks the TUI: only CLI subcommands
// consult the env var, so a non-CLI invocation stays NotHandled.
func TestExecuteInvalidCloudEnvIgnoredByTUI(t *testing.T) {
	old, had := os.LookupEnv("LAZYCLOUD_CLOUD")
	t.Setenv("LAZYCLOUD_CLOUD", "azure")
	defer func() {
		if had {
			os.Setenv("LAZYCLOUD_CLOUD", old)
		} else {
			os.Unsetenv("LAZYCLOUD_CLOUD")
		}
	}()
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Execute([]string{"lazycloud", "dashboard"}, stdout, stderr)
	if code != NotHandled {
		t.Fatalf("exit = %d, want NotHandled for TUI invocation", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

// TestSplitCloudFlagEmptyValueErrors covers an explicit empty flag
// value: naming --cloud with nothing after the equals sign is a
// usage error, not a silent gcp fallback.
func TestSplitCloudFlagEmptyValueErrors(t *testing.T) {
	for _, args := range [][]string{
		{"lazycloud", "list", "vms", "--cloud="},
		{"lazycloud", "list", "vms", "--cloud", ""},
	} {
		_, _, err := splitCloudFlag(args, "")
		if err == nil || !strings.Contains(err.Error(), "requires a value") {
			t.Fatalf("args %v: err = %v, want requires-a-value error", args, err)
		}
	}
}

// TestExecuteMissingCloudValue covers a dangling --cloud through the
// production entry.
func TestExecuteMissingCloudValue(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Execute([]string{"lazycloud", "list", "vms", "--cloud"}, stdout, stderr)
	if code != exitUsage {
		t.Fatalf("exit = %d, want usage failure", code)
	}
	if !strings.Contains(stderr.String(), "requires a value") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestDefaultDepsForCloudGCP(t *testing.T) {
	d, err := defaultDepsForCloud("")
	if err != nil {
		t.Fatalf("defaultDepsForCloud: %v", err)
	}
	if d.openCache == nil || d.fetchers.Projects == nil {
		t.Fatal("gcp deps must wire cache and fetchers")
	}
}

func TestDefaultDepsForCloudNebius(t *testing.T) {
	d, err := defaultDepsForCloud(cloudreg.Nebius)
	if err != nil {
		t.Fatalf("defaultDepsForCloud nebius: %v", err)
	}
	if d.fetchers.Projects == nil || d.fetchers.VMs == nil || d.fetchers.IAMBindings == nil {
		t.Fatal("nebius deps must wire fetchers")
	}
}

func TestDefaultDepsForCloudUnknown(t *testing.T) {
	_, err := defaultDepsForCloud("wat")
	if err == nil {
		t.Fatal("unknown cloud must error")
	}
}

// TestNebiusFetchersShape pins the Nebius adapter through the
// registry: every Fetchers entry non-nil, pointing at the provider
// functions.
func TestNebiusFetchersShape(t *testing.T) {
	p, err := cloudreg.Get(cloudreg.Nebius)
	if err != nil {
		t.Fatalf("cloud.Get(nebius): %v", err)
	}
	fields := reflect.ValueOf(p.Fetchers)
	for i := 0; i < fields.NumField(); i++ {
		if fields.Field(i).IsNil() {
			t.Errorf("nebius fetchers field %s is nil", fields.Type().Field(i).Name)
		}
	}
}

// TestNebiusListThroughInjectableDeps runs a full list invocation
// through the real nebius wiring: defaultDepsForCloud picks the
// nebius cache and fetcher set, then only the fetchers are swapped
// for fakes, so the invocation opens cache-nebius.db under a temp
// HOME and exercises the cloud-agnostic registry on top of it.
func TestNebiusListThroughInjectableDeps(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	d, err := defaultDepsForCloud(cloudreg.Nebius)
	if err != nil {
		t.Fatalf("defaultDepsForCloud: %v", err)
	}
	d.fetchers = fixedFetchers()
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := ExecuteDeps([]string{"lazycloud", "list", "projects", "--refresh", "--json"}, stdout, stderr, d)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "p1") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "lazycloud", "cache-nebius.db")); err != nil {
		t.Fatalf("list must go through the nebius cache: %v", err)
	}
}

// TestNebiusTableHeadersRenamedColumn proves the registry override
// reaches the printed table: the projects third column says Region,
// not Project Number, because Nebius fills it with the region.
func TestNebiusTableHeadersRenamedColumn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	d, err := defaultDepsForCloud(cloudreg.Nebius)
	if err != nil {
		t.Fatalf("defaultDepsForCloud: %v", err)
	}
	d.fetchers = fixedFetchers()
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := ExecuteDeps([]string{"lazycloud", "list", "projects", "--refresh", "--table"}, stdout, stderr, d)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Region") {
		t.Fatalf("table headers = %q, want Region column", out)
	}
	if strings.Contains(out, "Project Number") {
		t.Fatalf("table headers = %q, Project Number must not appear", out)
	}
}

// TestNebiusRefreshThroughInjectableDeps proves the refresh path on
// the real nebius wiring writes fetched data through cache-nebius.db
// and a later cache-only list reads it back from the same cache.
func TestNebiusRefreshThroughInjectableDeps(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	d, err := defaultDepsForCloud(cloudreg.Nebius)
	if err != nil {
		t.Fatalf("defaultDepsForCloud: %v", err)
	}
	d.fetchers = fixedFetchers()
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := ExecuteDeps([]string{"lazycloud", "list", "projects", "--refresh", "--json"}, stdout, stderr, d)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "one") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	stdout2, stderr2 := new(bytes.Buffer), new(bytes.Buffer)
	code = ExecuteDeps([]string{"lazycloud", "list", "projects", "--json"}, stdout2, stderr2, d)
	if code != exitOK {
		t.Fatalf("second exit = %d, stderr = %q", code, stderr2.String())
	}
	if !strings.Contains(stdout2.String(), "p1") {
		t.Fatalf("second stdout = %q", stdout2.String())
	}
}

// TestCloudFlagStrippedBeforeNouns proves --cloud survives stripping
// without confusing the noun parser: parsing runs on the rest slice.
func TestCloudFlagStrippedBeforeNouns(t *testing.T) {
	_, rest, err := splitCloudFlag([]string{"lazycloud", "--cloud", "nebius", "list", "vms", "--json"}, "")
	if err != nil {
		t.Fatalf("splitCloudFlag: %v", err)
	}
	if len(rest) != 4 || rest[1] != "list" || rest[2] != "vms" || rest[3] != "--json" {
		t.Fatalf("rest = %v", rest)
	}
}
