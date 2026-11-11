package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func TestToSnake(t *testing.T) {
	cases := map[string]string{
		"ProjectID":       "project_id",
		"Name":            "name",
		"IPRange":         "ip_range",
		"SizeGB":          "size_gb",
		"InternalIP":      "internal_ip",
		"VMDisk":          "vm_disk",
		"ServiceAccounts": "service_accounts",
		"NetworkTags":     "network_tags",
		"MTU":             "mtu",
		"A":               "a",
	}
	for name, want := range cases {
		if got := toSnake(name); got != want {
			t.Errorf("toSnake(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestSnakeValueStruct(t *testing.T) {
	vm := providers.VM{
		ProjectID:  "p1",
		Name:       "vm-01",
		BootDisk:   &providers.VMDisk{Name: "boot"},
		Interfaces: []providers.VMInterface{{InternalIP: "10.0.0.5", ExternalIP: "34.0.0.1"}},
		Labels:     map[string]string{"MyLabel": "v"},
	}
	m, ok := snakeValue(vm).(map[string]any)
	if !ok {
		t.Fatalf("snakeValue(vm) = %T, want map", snakeValue(vm))
	}
	for _, key := range []string{"project_id", "name", "boot_disk", "interfaces", "labels"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
	if m["project_id"] != "p1" {
		t.Errorf("project_id = %v", m["project_id"])
	}
	ifaces, ok := m["interfaces"].([]any)
	if !ok || len(ifaces) != 1 {
		t.Fatalf("interfaces = %T (%v)", m["interfaces"], m["interfaces"])
	}
	if iface := ifaces[0].(map[string]any); iface["internal_ip"] != "10.0.0.5" {
		t.Errorf("internal_ip = %v", iface["internal_ip"])
	}
	disk, ok := m["boot_disk"].(map[string]any)
	if !ok {
		t.Fatalf("boot_disk = %T", m["boot_disk"])
	}
	if disk["name"] != "boot" {
		t.Errorf("boot_disk.name = %v", disk["name"])
	}
	// Map keys pass through untouched: a user label keeps its case.
	labels := m["labels"].(map[string]any)
	if _, ok := labels["MyLabel"]; !ok {
		t.Errorf("label key must survive: %v", labels)
	}
}

func TestSnakeValueNilVariants(t *testing.T) {
	m := snakeValue(providers.VM{}).(map[string]any)
	for _, key := range []string{"labels", "boot_disk", "interfaces", "network_tags"} {
		if m[key] != nil {
			t.Errorf("%s = %v, want nil", key, m[key])
		}
	}
}

func TestSnakeValueScalars(t *testing.T) {
	if got := snakeValue(nil); got != nil {
		t.Errorf("nil = %v", got)
	}
	if got := snakeValue("s"); got != "s" {
		t.Errorf("string = %v", got)
	}
	if got := snakeValue(42); got != 42 {
		t.Errorf("int = %v", got)
	}
	if got := snakeValue(true); got != true {
		t.Errorf("bool = %v", got)
	}
}

func TestSnakeValueTime(t *testing.T) {
	now := time.Now()
	got := snakeValue(now)
	s, ok := got.(string)
	if !ok {
		t.Fatalf("time.Time = %T, want string", got)
	}
	if s != now.Format(time.RFC3339Nano) {
		t.Errorf("time = %q", s)
	}
}

func TestSnakeValuePtr(t *testing.T) {
	n := 7
	if got := snakeValue(&n); got != 7 {
		t.Errorf("ptr deref = %v", got)
	}
}

func TestSnakeValueUnexportedFieldsSkipped(t *testing.T) {
	type mixed struct {
		Public int
		hidden int
	}
	m, ok := snakeValue(mixed{Public: 1, hidden: 2}).(map[string]any)
	if !ok {
		t.Fatalf("snakeValue = %T, want map", m)
	}
	if m["public"] != 1 {
		t.Errorf("public = %v", m["public"])
	}
	if _, exists := m["hidden"]; exists {
		t.Error("unexported fields must be skipped")
	}
}

func TestWriteOne(t *testing.T) {
	var buf bytes.Buffer
	if err := writeOne(providers.Project{ProjectID: "p1", Name: "one"}, &buf); err != nil {
		t.Fatalf("writeOne: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("writeOne output not JSON: %v", err)
	}
	if got["project_id"] != "p1" || got["name"] != "one" {
		t.Errorf("got = %v", got)
	}
}

func TestWriteStructuredYAML(t *testing.T) {
	var buf bytes.Buffer
	v := snakeValue([]any{providers.Project{ProjectID: "p1"}})
	if err := writeStructured(v, formatYAML, &buf); err != nil {
		t.Fatalf("writeStructured: %v", err)
	}
	if !strings.Contains(buf.String(), "project_id: p1") {
		t.Errorf("yaml missing key:\n%s", buf.String())
	}
}

func TestRenderItemsAllFormats(t *testing.T) {
	def := resourceDef{headers: []string{"Project ID"}, row: projectRow}
	items := []any{providers.Project{ProjectID: "p1"}}
	for _, format := range []string{formatJSON, formatYAML, formatNDJSON, formatTable} {
		var buf bytes.Buffer
		if err := renderItems(items, format, def, &buf); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if buf.Len() == 0 {
			t.Errorf("%s: empty output", format)
		}
		if format == formatJSON && !json.Valid(buf.Bytes()) {
			t.Errorf("%s: invalid json", format)
		}
	}
}

func TestRenderItemsTableEmpty(t *testing.T) {
	def := resourceDef{headers: []string{"Project ID"}, row: projectRow}
	var buf bytes.Buffer
	if err := renderItems(nil, formatTable, def, &buf); err != nil {
		t.Fatalf("renderItems: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "Project ID") {
		t.Errorf("empty table should still print headers: %q", got)
	}
}

func TestRenderItemsNDJSONMultiple(t *testing.T) {
	def := resourceDef{headers: []string{"Project ID"}, row: projectRow}
	items := []any{
		providers.Project{ProjectID: "p1"},
		providers.Project{ProjectID: "p2"},
	}
	var buf bytes.Buffer
	if err := renderItems(items, formatNDJSON, def, &buf); err != nil {
		t.Fatalf("renderItems: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("ndjson lines = %d, want 2", len(lines))
	}
	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("line 2 not JSON: %v", err)
	}
	if second["project_id"] != "p2" {
		t.Errorf("line 2 = %v", second)
	}
}

func TestRenderSingleJSON(t *testing.T) {
	def := resourceDef{headers: []string{"Project ID"}, row: projectRow}
	var buf bytes.Buffer
	if err := renderSingle(providers.Project{ProjectID: "p1"}, formatJSON, def, &buf); err != nil {
		t.Fatalf("renderSingle: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if got["project_id"] != "p1" {
		t.Errorf("got = %v", got)
	}
}

func TestRenderSingleTableAndNDJSON(t *testing.T) {
	def := resourceDef{headers: []string{"Project ID", "Name", "Project Number"}, row: projectRow}
	var buf bytes.Buffer
	if err := renderSingle(providers.Project{ProjectID: "p1"}, formatTable, def, &buf); err != nil {
		t.Fatalf("table: %v", err)
	}
	if !strings.Contains(buf.String(), "p1") {
		t.Errorf("table missing p1:\n%s", buf.String())
	}
	buf.Reset()
	if err := renderSingle(providers.Project{ProjectID: "p1"}, formatNDJSON, def, &buf); err != nil {
		t.Fatalf("ndjson: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("ndjson not JSON: %v", err)
	}
	if got["project_id"] != "p1" {
		t.Errorf("ndjson = %v", got)
	}
}

func TestWriteTableAlignment(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"A", "BB"}
	rows := [][]string{{"xyz", "b"}, {"a", "longer"}}
	if err := writeTable(headers, rows, &buf); err != nil {
		t.Fatalf("writeTable: %v", err)
	}
	want := "A    BB\nxyz  b\na    longer\n"
	if got := buf.String(); got != want {
		t.Errorf("table = %q, want %q", got, want)
	}
}

func TestWriteTableNoRows(t *testing.T) {
	var buf bytes.Buffer
	if err := writeTable([]string{"A"}, nil, &buf); err != nil {
		t.Fatalf("writeTable: %v", err)
	}
	if got := buf.String(); got != "A\n" {
		t.Errorf("table = %q, want %q", got, "A\n")
	}
}

func TestWriteTableShortRow(t *testing.T) {
	// A row with fewer cells than headers must not panic.
	var buf bytes.Buffer
	if err := writeTable([]string{"A", "B"}, [][]string{{"only"}}, &buf); err != nil {
		t.Fatalf("writeTable: %v", err)
	}
	if !strings.Contains(buf.String(), "only") {
		t.Errorf("table = %q", buf.String())
	}
}

func TestWriteTableLongRow(t *testing.T) {
	// A row longer than headers ignores extra cells.
	var buf bytes.Buffer
	if err := writeTable([]string{"A"}, [][]string{{"x", "y"}}, &buf); err != nil {
		t.Fatalf("writeTable: %v", err)
	}
	if got := buf.String(); !strings.HasPrefix(got, "A\nx") {
		t.Errorf("table = %q", got)
	}
}

func TestFormatUpdatedAt(t *testing.T) {
	if got := formatUpdatedAt(""); got != "" {
		t.Errorf("empty = %q", got)
	}
	if got := formatUpdatedAt("not-a-time"); got != "not-a-time" {
		t.Errorf("unparsable must stay raw: %q", got)
	}
	if got := formatUpdatedAt("2026-01-02T10:00:00Z"); got == "2026-01-02T10:00:00Z" {
		t.Errorf("RFC3339 must be localized: %q", got)
	}
}

func TestFormatFlagMapping(t *testing.T) {
	for arg, want := range map[string]string{
		"--json":   formatJSON,
		"--yaml":   formatYAML,
		"--ndjson": formatNDJSON,
		"--table":  formatTable,
	} {
		if got := formatFlag(arg); got != want {
			t.Errorf("formatFlag(%q) = %q, want %q", arg, got, want)
		}
	}
	if got := formatFlag("--nope"); got != "" {
		t.Errorf("formatFlag(--nope) = %q, want empty", got)
	}
}

func TestParseFlagsProjectForms(t *testing.T) {
	env := func() string { return "" }
	for _, tt := range []struct {
		args     []string
		wantProj string
		wantErr  bool
	}{
		{args: []string{"--project", "p1"}, wantProj: "p1"},
		{args: []string{"--project=p1"}, wantProj: "p1"},
		{args: []string{"--project"}, wantErr: true},
	} {
		opts, _, err := parseFlags("list", tt.args, env)
		if tt.wantErr {
			if err == nil {
				t.Errorf("args %v: want error", tt.args)
			}
			continue
		}
		if err != nil {
			t.Fatalf("args %v: %v", tt.args, err)
		}
		if opts.Project != tt.wantProj {
			t.Errorf("args %v: project = %q, want %q", tt.args, opts.Project, tt.wantProj)
		}
	}
}

func TestParseFlagsEnvProjectDefault(t *testing.T) {
	opts, _, err := parseFlags("list", nil, func() string { return "from-env" })
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if opts.Project != "from-env" {
		t.Errorf("project = %q, want from-env", opts.Project)
	}
}

func TestParseFlagsConflictingFormats(t *testing.T) {
	env := func() string { return "" }
	if _, _, err := parseFlags("list", []string{"--json", "--yaml"}, env); err == nil {
		t.Error("json then yaml must conflict")
	}
	if _, _, err := parseFlags("list", []string{"--yaml", "--json"}, env); err == nil {
		t.Error("yaml then json must conflict")
	}
}

func TestParseFlagsRepeatedSameFormatOK(t *testing.T) {
	opts, _, err := parseFlags("list", []string{"--table", "--table"}, func() string { return "" })
	if err != nil {
		t.Fatalf("same format twice: %v", err)
	}
	if opts.Format != formatTable {
		t.Errorf("format = %q, want table", opts.Format)
	}
}

func TestParseFlagsRefreshOnGetRejected(t *testing.T) {
	if _, _, err := parseFlags("get", []string{"p1/z/vm", "--refresh"}, func() string { return "" }); err == nil {
		t.Error("--refresh on get must be rejected")
	}
}

func TestParseFlagsListTakesNoSelector(t *testing.T) {
	if _, _, err := parseFlags("list", []string{"p1"}, func() string { return "" }); err == nil {
		t.Error("list with positional arg must error")
	}
}

func TestParseFlagsGetSelectorSplitting(t *testing.T) {
	_, sel, err := parseFlags("get", []string{"p1/europe-west1-b/vm-01"}, func() string { return "" })
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	want := []string{"p1", "europe-west1-b", "vm-01"}
	if !reflect.DeepEqual(sel, want) {
		t.Errorf("sel = %v, want %v", sel, want)
	}
}

func TestParseFlagsGetMultipleSelectorArgs(t *testing.T) {
	_, sel, err := parseFlags("get", []string{"p1", "z", "vm-01"}, func() string { return "" })
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	want := []string{"p1", "z", "vm-01"}
	if !reflect.DeepEqual(sel, want) {
		t.Errorf("sel = %v, want %v", sel, want)
	}
}

func TestParseFlagsUnknownFlag(t *testing.T) {
	if _, _, err := parseFlags("list", []string{"--frob"}, func() string { return "" }); err == nil {
		t.Error("unknown flag must error")
	}
}
