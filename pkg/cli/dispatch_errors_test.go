package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// errWriter fails every write, standing in for a closed pipe.
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, errors.New("pipe closed") }

// closedCacheDeps returns deps whose openCache yields a cache with a
// closed database, so cache reads fail like real infrastructure
// failures.
func closedCacheDeps(t *testing.T, fetchers Fetchers) (deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	d, stdout, stderr := testDeps(t, fetchers)
	d.openCache = func() (*providers.Cache, error) {
		c, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "c.db"))
		if err != nil {
			return nil, err
		}
		c.Close()
		return c, nil
	}
	return d, stdout, stderr
}

func TestExecuteViaProductionEntry(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	if code := Execute([]string{"lazycloud", "resources"}, stdout, stderr); code != exitOK {
		t.Fatalf("Execute exit = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "projects") {
		t.Errorf("resources = %q", stdout.String())
	}
}

func TestDefaultDepsEnvProject(t *testing.T) {
	if got, want := defaultDeps().envProject(), os.Getenv("LAZYCLOUD_PROJECT"); got != want {
		t.Errorf("envProject = %q, want %q", got, want)
	}
}

func TestDefaultDepsSaveViewUpdate(t *testing.T) {
	cache, err := providers.NewCacheAt(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("NewCacheAt: %v", err)
	}
	defer cache.Close()
	if err := defaultDeps().saveViewUpdate(cache, "vms", "p1"); err != nil {
		t.Errorf("saveViewUpdate: %v", err)
	}
}

func TestExecuteRefreshTargetsCacheError(t *testing.T) {
	d, _, stderr := closedCacheDeps(t, fixedFetchers())
	args := []string{"lazycloud", "list", "vms", "--refresh"}
	if code := ExecuteDeps(args, new(bytes.Buffer), stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "refresh:") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestExecuteListCacheReadError(t *testing.T) {
	d, _, stderr := closedCacheDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "list", "projects"}, new(bytes.Buffer), stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "reading cache:") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestExecuteGetCacheError(t *testing.T) {
	d, _, stderr := closedCacheDeps(t, fixedFetchers())
	args := []string{"lazycloud", "get", "project", "p1"}
	if code := ExecuteDeps(args, new(bytes.Buffer), stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "get:") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestExecuteRefreshGlobalFetchError(t *testing.T) {
	boomer := fixedFetchers()
	boomer.Projects = func() ([]providers.Project, error) {
		return nil, errors.New("gcloud down")
	}
	d, _, stderr := testDeps(t, boomer)
	if code := ExecuteDeps([]string{"lazycloud", "projects", "--refresh"}, new(bytes.Buffer), stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "gcloud down") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestExecuteRefreshNetworksFirstFetchError(t *testing.T) {
	boomer := fixedFetchers()
	boomer.Networks = func(string) ([]providers.Network, error) {
		return nil, errors.New("networks down")
	}
	d, _, stderr := testDeps(t, boomer)
	args := []string{"lazycloud", "list", "networks", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(args, new(bytes.Buffer), stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "networks down") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestExecuteRefreshNetworksSecondFetchError(t *testing.T) {
	boomer := fixedFetchers()
	boomer.Subnetworks = func(string) ([]providers.Subnetwork, error) {
		return nil, errors.New("subnets down")
	}
	d, _, stderr := testDeps(t, boomer)
	args := []string{"lazycloud", "list", "networks", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(args, new(bytes.Buffer), stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "subnets down") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestExecuteRefreshScopedNouns(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	for _, noun := range []string{"disks", "clusters", "buckets"} {
		stdout.Reset()
		args := []string{"lazycloud", "list", noun, "--project", "p1", "--refresh"}
		if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
			t.Fatalf("%s refresh exit = %d; stderr: %s", noun, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), `"name"`) {
			t.Errorf("%s refresh output = %q", noun, stdout.String())
		}
	}
}

func TestExecuteGetNotFoundPaths(t *testing.T) {
	cases := [][]string{
		{"lazycloud", "get", "project", "missing"},
		{"lazycloud", "get", "disk", "p1/z/missing"},
		{"lazycloud", "get", "cluster", "p1/missing"},
		{"lazycloud", "get", "bucket", "p1/missing"},
		{"lazycloud", "get", "networks", "p1/missing"},
		{"lazycloud", "get", "sa", "p1/missing@p1.iam"},
		{"lazycloud", "get", "iam", "p1/user:miss/roles/viewer"},
	}
	for _, args := range cases {
		d, _, stderr := testDeps(t, fixedFetchers())
		if code := ExecuteDeps(args, new(bytes.Buffer), stderr, d); code != exitNotFound {
			t.Errorf("args %v exit = %d, want 2; stderr: %s", args, code, stderr.String())
		}
	}
}

func TestExecuteGetHappyPaths(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "disks", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("disk seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "get", "disk", "p1/europe-west1-b/disk-01"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("disk get exit = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name":"disk-01"`) {
		t.Errorf("disk get = %q", stdout.String())
	}

	seed = []string{"lazycloud", "list", "clusters", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("cluster seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "get", "cluster", "p1/cluster-01"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("cluster get exit = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name":"cluster-01"`) {
		t.Errorf("cluster get = %q", stdout.String())
	}

	seed = []string{"lazycloud", "list", "buckets", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("bucket seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "get", "bucket", "p1/bucket-01"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("bucket get exit = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name":"bucket-01"`) {
		t.Errorf("bucket get = %q", stdout.String())
	}

	seed = []string{"lazycloud", "list", "networks", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("network seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "get", "networks", "p1/net-01"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("vpc get exit = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name":"net-01"`) {
		t.Errorf("vpc get = %q", stdout.String())
	}
}

func TestExecuteListRenderError(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	args := []string{"lazycloud", "list", "vms", "--project", "p1"}
	if code := ExecuteDeps(args, errWriter{}, stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "rendering:") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestExecuteGetRenderError(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	args := []string{"lazycloud", "get", "vm", "p1/europe-west1-b/vm-01"}
	if code := ExecuteDeps(args, errWriter{}, stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "rendering:") {
		t.Errorf("stderr = %q", stderr.String())
	}
}
