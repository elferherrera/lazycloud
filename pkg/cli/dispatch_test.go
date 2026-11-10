package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func TestExecuteResources(t *testing.T) {
	d, stdout, stderr := testDeps(t, Fetchers{})
	if code := ExecuteDeps([]string{"lazycloud", "resources"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := "buckets clusters disks iam networks projects serviceaccounts vms"
	if got := strings.TrimSuffix(stdout.String(), "\n"); got != want {
		t.Errorf("resources = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestExecuteNotHandled(t *testing.T) {
	d, _, _ := testDeps(t, Fetchers{})
	for _, args := range [][]string{
		{"lazycloud"},
		{"lazycloud", "not-a-command"},
		{"lazycloud", "--version"},
	} {
		if code := ExecuteDeps(args, new(bytes.Buffer), new(bytes.Buffer), d); code != NotHandled {
			t.Errorf("args %v = %d, want NotHandled", args, code)
		}
	}
}

func TestExecuteProjectsShortcut(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "projects", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v\nstdout: %s", err, stdout.String())
	}
	if len(got) != 1 || got[0]["project_id"] != "p1" {
		t.Errorf("projects = %v", got)
	}
}

func TestExecuteListDefaultJSON(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	args := []string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v\nstdout: %s", err, stdout.String())
	}
	if len(got) != 1 || got[0]["name"] != "vm-01" {
		t.Errorf("vms = %v", got)
	}
}

func TestExecuteListDefaultJSONNoProject(t *testing.T) {
	// Without --project a global list covers all cached projects.
	d, stdout, stderr := testDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "projects", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "list", "projects"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("projects = %v", got)
	}
}

func TestExecuteListScopedCacheRead(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms", "--project", "p1"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if len(got) != 1 || got[0]["project_id"] != "p1" {
		t.Errorf("vms = %v", got)
	}
}

func TestExecuteListAllCachedProjects(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if len(got) != 1 || got[0]["name"] != "vm-01" {
		t.Errorf("vms = %v", got)
	}
}

func TestExecuteListWithEnvProject(t *testing.T) {
	f := fixedFetchers()
	f.VMs = func(string) ([]providers.VM, error) {
		return []providers.VM{{ProjectID: "env-pro", Name: "vm-env"}}, nil
	}
	d, stdout, stderr := testDeps(t, f)
	d.envProject = func() string { return "env-pro" }
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if len(got) != 1 || got[0]["project_id"] != "env-pro" {
		t.Errorf("vms = %v", got)
	}
}

func TestExecuteListTable(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	args := []string{"lazycloud", "list", "vms", "--project", "p1", "--refresh", "--table"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"Name", "Zone", "Machine Type", "Status", "Internal IPs", "Updated", "vm-01", "10.0.0.5"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestExecuteListYAML(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "projects", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "list", "projects", "--yaml"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "project_id: p1") {
		t.Errorf("yaml missing snake_case key:\n%s", stdout.String())
	}
}

func TestExecuteListNDJSON(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "projects", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "list", "projects", "--ndjson"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("ndjson lines = %d, want 1", len(lines))
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("line not JSON: %v", err)
	}
	if got["project_id"] != "p1" {
		t.Errorf("project_id = %v, want p1", got["project_id"])
	}
}

func TestExecuteListUnknownResource(t *testing.T) {
	d, stdout, stderr := testDeps(t, Fetchers{})
	if code := ExecuteDeps([]string{"lazycloud", "list", "dragons"}, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "valid:") {
		t.Errorf("stderr should list valid resources: %q", stderr.String())
	}
}

func TestExecuteListMissingResource(t *testing.T) {
	d, stdout, stderr := testDeps(t, Fetchers{})
	if code := ExecuteDeps([]string{"lazycloud", "list"}, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestExecuteListUnknownFlag(t *testing.T) {
	d, stdout, stderr := testDeps(t, Fetchers{})
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms", "--frob"}, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestExecuteListRefreshRecordsViewUpdate(t *testing.T) {
	var records []string
	d, stdout, stderr := testDeps(t, fixedFetchers())
	d.saveViewUpdate = func(_ *providers.Cache, view, projectID string) error {
		records = append(records, view+"@"+projectID)
		return nil
	}
	args := []string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !reflect.DeepEqual(records, []string{"vms@p1"}) {
		t.Errorf("view updates = %v, want [vms@p1]", records)
	}
}

func TestExecuteListRefreshGlobalRecordsOneTarget(t *testing.T) {
	var records []string
	d, stdout, stderr := testDeps(t, fixedFetchers())
	d.saveViewUpdate = func(_ *providers.Cache, view, projectID string) error {
		records = append(records, view+"@"+projectID)
		return nil
	}
	if code := ExecuteDeps([]string{"lazycloud", "projects", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !reflect.DeepEqual(records, []string{"projects@"}) {
		t.Errorf("view updates = %v, want [projects@]", records)
	}
}

func TestExecuteListRefreshAllProjects(t *testing.T) {
	var refreshed []string
	f := fixedFetchers()
	f.VMs = func(projectID string) ([]providers.VM, error) {
		refreshed = append(refreshed, projectID)
		return []providers.VM{{ProjectID: projectID, Name: "vm-" + projectID}}, nil
	}
	d, stdout, stderr := testDeps(t, f)
	if code := ExecuteDeps([]string{"lazycloud", "projects", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !reflect.DeepEqual(refreshed, []string{"p1"}) {
		t.Errorf("refreshed projects = %v, want [p1]", refreshed)
	}
}

func TestExecuteListRefreshNoProjectsCached(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms", "--refresh"}, stdout, stderr, d); code != exitNotFound {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "projects --refresh") {
		t.Errorf("stderr should point at the fix: %q", stderr.String())
	}
}

func TestExecuteListRefreshFetchError(t *testing.T) {
	boomer := fixedFetchers()
	boomer.VMs = func(string) ([]providers.VM, error) {
		return nil, errors.New("quota exceeded")
	}
	d, stdout, stderr := testDeps(t, boomer)
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}, stdout, stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "quota exceeded") {
		t.Errorf("stderr should carry the fetch error: %q", stderr.String())
	}
}

func TestExecuteListRefreshRecordWarningOnly(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	d.saveViewUpdate = func(_ *providers.Cache, _, _ string) error {
		return errors.New("db locked")
	}
	args := []string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0 (warning only); stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning: recording view update") {
		t.Errorf("stderr should carry the warning: %q", stderr.String())
	}
}

func TestExecuteGetJSON(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	args := []string{"lazycloud", "get", "vm", "p1/europe-west1-b/vm-01"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if got["machine_type"] != "e2-small" {
		t.Errorf("machine_type = %v, want e2-small", got["machine_type"])
	}
}

func TestExecuteGetNotFound(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	args := []string{"lazycloud", "get", "vm", "p1/europe-west1-b/missing"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitNotFound {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestExecuteGetBadSelectorHint(t *testing.T) {
	d, stdout, stderr := testDeps(t, Fetchers{})
	args := []string{"lazycloud", "get", "vm", "p1"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "project/zone/name") {
		t.Errorf("error should hint the selector shape: %q", stderr.String())
	}
}

func TestExecuteGetMissingSelector(t *testing.T) {
	d, stdout, stderr := testDeps(t, Fetchers{})
	if code := ExecuteDeps([]string{"lazycloud", "get", "vms"}, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestExecuteGetRejectsRefresh(t *testing.T) {
	d, stdout, stderr := testDeps(t, Fetchers{})
	args := []string{"lazycloud", "get", "vm", "p1/z/vm-01", "--refresh"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestExecuteCacheOpenError(t *testing.T) {
	d, _, stderr := testDeps(t, Fetchers{})
	d.openCache = func() (*providers.Cache, error) { return nil, errors.New("locked") }
	if code := ExecuteDeps([]string{"lazycloud", "list", "projects"}, new(bytes.Buffer), stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "opening cache") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestExecuteUnknownResourceNoCacheOpen(t *testing.T) {
	// A usage error must open no cache: a broken cache path must not
	// turn a usage error into an infrastructure error.
	d, _, _ := testDeps(t, Fetchers{})
	d.openCache = func() (*providers.Cache, error) { return nil, errors.New("locked") }
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	if code := ExecuteDeps([]string{"lazycloud", "list", "dragons"}, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestExecuteScopedListEmptyForUncachedProject(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "list", "vms", "--project", "ghost"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0 (empty is success)", code)
	}
	if strings.TrimSpace(stdout.String()) != "[]" {
		t.Errorf("stdout = %q, want []", stdout.String())
	}
}

func TestExecuteNetworksSubnetGetAndList(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "networks", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "list", "networks", "--project", "p1"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	// Networks list combines VPCs and subnets: two rows.
	if len(got) != 2 {
		t.Fatalf("networks = %d rows, want 2: %v", len(got), got)
	}
	stdout.Reset()
	sub := []string{"lazycloud", "get", "networks", "p1/europe-west1/subnet-01"}
	if code := ExecuteDeps(sub, stdout, stderr, d); code != exitOK {
		t.Fatalf("subnet get exit = %d; stderr: %s", code, stderr.String())
	}
	var one map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &one); err != nil {
		t.Fatalf("subnet get not JSON: %v", err)
	}
	if one["ip_range"] != "10.0.0.0/24" {
		t.Errorf("ip_range = %v", one["ip_range"])
	}
}

func TestExecuteIAMBindingGet(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "iam", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	// The role contains a slash: parts after the member rejoin.
	args := []string{"lazycloud", "get", "iam", "p1/user:x@example.com/roles/viewer"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if got["role"] != "roles/viewer" {
		t.Errorf("role = %v, want roles/viewer", got["role"])
	}
}

func TestExecuteIAMBindingBadSelectorHint(t *testing.T) {
	d, stdout, stderr := testDeps(t, Fetchers{})
	// Only project/member: missing the role part.
	args := []string{"lazycloud", "get", "iam", "p1/user:x@example.com"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "project/member/role") {
		t.Errorf("stderr should hint project/member/role: %q", stderr.String())
	}
}

func TestExecuteAliasNouns(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "vms", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "list", "vm", "--project", "p1"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("alias vm exit = %d; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("vm alias = %v", got)
	}
}

func TestExecuteScopedListOnGlobalResourceIgnoresProject(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	if code := ExecuteDeps([]string{"lazycloud", "projects", "--refresh"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	args := []string{"lazycloud", "list", "projects", "--project", "whatever"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("projects = %v, want global list regardless of scope", got)
	}
}

func TestDefaultDepsWiresProduction(t *testing.T) {
	d := defaultDeps()
	if d.openCache == nil || d.envProject == nil || d.saveViewUpdate == nil {
		t.Fatal("defaultDeps must wire openCache, envProject, saveViewUpdate")
	}
	if d.fetchers.Projects == nil {
		t.Fatal("defaultDeps must wire real fetchers")
	}
}

func TestExecuteProjectsShortcutTable(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	args := []string{"lazycloud", "projects", "--refresh", "--table"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "p1") {
		t.Errorf("table missing p1:\n%s", stdout.String())
	}
}

func TestExecuteVMsTableHeadersMatch(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	args := []string{"lazycloud", "list", "vms", "--project", "p1", "--refresh", "--table"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	firstLine := strings.SplitN(stdout.String(), "\n", 2)[0]
	// Column padding varies with cell widths; compare the words.
	got := strings.Fields(firstLine)
	want := []string{"Name", "Zone", "Machine", "Type", "Status", "Internal", "IPs", "Updated"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("header = %q, want %v", firstLine, want)
	}
}

func TestExecuteNetworksBadSelectorHint(t *testing.T) {
	d, stdout, stderr := testDeps(t, Fetchers{})
	args := []string{"lazycloud", "get", "networks", "p1/europe-west1/subnet-01/extra"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "project/name or project/region/name") {
		t.Errorf("stderr should hint the two network forms: %q", stderr.String())
	}
}

func TestExecuteServiceAccountGet(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "serviceaccounts", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	args := []string{"lazycloud", "get", "sa", "p1/sa-01@p1.iam.gserviceaccount.com"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if got["email"] != "sa-01@p1.iam.gserviceaccount.com" {
		t.Errorf("email = %v", got["email"])
	}
}

func TestExecuteGetTable(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "projects", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	args := []string{"lazycloud", "get", "project", "p1", "--table"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "p1") {
		t.Errorf("table missing p1:\n%s", stdout.String())
	}
}

func TestExecuteGetCacheOpenError(t *testing.T) {
	d, _, stderr := testDeps(t, Fetchers{})
	d.openCache = func() (*providers.Cache, error) { return nil, errors.New("locked") }
	args := []string{"lazycloud", "get", "project", "p1"}
	if code := ExecuteDeps(args, new(bytes.Buffer), stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
}
