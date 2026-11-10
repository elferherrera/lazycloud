package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

// oneShotWriter accepts the first write and fails every later one, so
// a table header lands but the first row write explodes.
type oneShotWriter struct {
	writes int
}

func (w *oneShotWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, errors.New("pipe closed")
	}
	return len(p), nil
}

func TestCombineAllBothLists(t *testing.T) {
	l1 := func() ([]providers.Network, error) {
		return []providers.Network{{Name: "n1"}}, nil
	}
	l2 := func() ([]providers.Subnetwork, error) {
		return []providers.Subnetwork{{Name: "s1"}}, nil
	}
	got, err := combineAll(l1, l2)
	if err != nil {
		t.Fatalf("combineAll: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("combineAll = %d items, want 2", len(got))
	}
	if _, ok := got[0].(providers.Network); !ok {
		t.Errorf("item 0 = %T", got[0])
	}
	if _, ok := got[1].(providers.Subnetwork); !ok {
		t.Errorf("item 1 = %T", got[1])
	}
}

func TestCombineAllFirstError(t *testing.T) {
	l1 := func() ([]providers.Network, error) { return nil, errors.New("boom") }
	l2 := func() ([]providers.Subnetwork, error) { return nil, nil }
	if _, err := combineAll(l1, l2); err == nil {
		t.Error("first list error must propagate")
	}
}

func TestCombineAllSecondError(t *testing.T) {
	l1 := func() ([]providers.Network, error) { return nil, nil }
	l2 := func() ([]providers.Subnetwork, error) { return nil, errors.New("boom") }
	if _, err := combineAll(l1, l2); err == nil {
		t.Error("second list error must propagate")
	}
}

func TestExecuteListNetworksAllProjects(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "networks", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	// No --project: the global list combines VPCs and subnets.
	if code := ExecuteDeps([]string{"lazycloud", "list", "networks"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("networks = %d rows, want 2", len(got))
	}
}

func TestExecuteGetSubnetNotFound(t *testing.T) {
	d, stdout, stderr := testDeps(t, fixedFetchers())
	seed := []string{"lazycloud", "list", "networks", "--project", "p1", "--refresh"}
	if code := ExecuteDeps(seed, stdout, stderr, d); code != exitOK {
		t.Fatalf("seed failed: %d; stderr: %s", code, stderr.String())
	}
	args := []string{"lazycloud", "get", "networks", "p1/europe-west1/missing"}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitNotFound {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRenderItemsNDJSONWriteError(t *testing.T) {
	def := resourceDef{headers: []string{"Name"}, row: projectRow}
	items := []any{providers.Project{ProjectID: "p1"}}
	if err := renderItems(items, formatNDJSON, def, errWriter{}); err == nil {
		t.Error("ndjson write failure must propagate")
	}
}

func TestRenderItemsTableWriteErrorHeaders(t *testing.T) {
	def := resourceDef{headers: []string{"Name"}, row: projectRow}
	items := []any{providers.Project{ProjectID: "p1"}}
	if err := renderItems(items, formatTable, def, errWriter{}); err == nil {
		t.Error("table header write failure must propagate")
	}
}

func TestRenderItemsTableWriteErrorRow(t *testing.T) {
	def := resourceDef{headers: []string{"Name"}, row: projectRow}
	items := []any{providers.Project{ProjectID: "p1"}}
	// The header write succeeds, the row write fails.
	if err := renderItems(items, formatTable, def, &oneShotWriter{}); err == nil {
		t.Error("table row write failure must propagate")
	}
}

func TestWriteOneMarshalError(t *testing.T) {
	// Channels survive snakeValue and break json.Marshal.
	var buf bytes.Buffer
	if err := writeOne(make(chan int), &buf); err == nil {
		t.Error("unmarshalable value must error")
	}
}

// failingYAML implements yaml.Marshaler with an error, the one input
// that makes yaml.Marshal return an error instead of panicking.
type failingYAML struct{}

func (failingYAML) MarshalYAML() (any, error) { return nil, errors.New("boom") }

func TestWriteStructuredMarshalErrors(t *testing.T) {
	var buf bytes.Buffer
	// Channels survive snakeValue and break json.Marshal.
	if err := writeStructured(make(chan int), formatJSON, &buf); err == nil {
		t.Error("json marshal failure must propagate")
	}
	if err := writeStructured(failingYAML{}, formatYAML, &buf); err == nil {
		t.Error("yaml marshal failure must propagate")
	}
}

func TestWriteStructuredYAMLWriteError(t *testing.T) {
	v := snakeValue([]any{providers.Project{ProjectID: "p1"}})
	if err := writeStructured(v, formatYAML, errWriter{}); err == nil {
		t.Error("yaml write failure must propagate")
	}
}

func TestOneShotWriterContract(t *testing.T) {
	var buf oneShotWriter
	if _, err := buf.Write([]byte("a")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := buf.Write([]byte("b")); err == nil {
		t.Fatal("second write must fail")
	}
}

func TestSnakeValueArray(t *testing.T) {
	got := snakeValue([2]int{1, 2})
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("snakeValue(array) = %T, want slice", got)
	}
	if len(arr) != 2 || arr[0] != 1 || arr[1] != 2 {
		t.Errorf("array = %v", arr)
	}
}
