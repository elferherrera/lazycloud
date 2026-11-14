package cli

import (
	"strings"
	"testing"
)

func TestHelpTextListsSurface(t *testing.T) {
	got := helpText("v1.2.3")
	for _, want := range []string{
		"lazycloud list <resource>",
		"lazycloud get <resource>",
		"lazycloud resources",
		"lazycloud projects",
		"--project",
		"LAZYCLOUD_PROJECT",
		"--refresh",
		"--json",
		"--ndjson",
		"--yaml",
		"--table",
		"Exit codes: 0 ok, 1 usage, 2 not found, 3 infrastructure error.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("help missing %q:\n%s", want, got)
		}
	}
}

func TestHelpTextListsEveryResource(t *testing.T) {
	got := helpText("v1.2.3")
	for _, noun := range ResourceNames() {
		if !strings.Contains(got, noun) {
			t.Errorf("help missing resource %q", noun)
		}
	}
}

func TestHelpTextSelectorsMatchRegistry(t *testing.T) {
	// The selector line must document every registry noun's selector
	// hint, so the help cannot drift from the resource definitions.
	got := helpText("v1.2.3")
	registry := defaultRegistry(Fetchers{})
	for _, noun := range ResourceNames() {
		if !strings.Contains(got, registry[noun].selectorHint) {
			t.Errorf("help missing selector hint %q for %s", registry[noun].selectorHint, noun)
		}
	}
}

func TestHelpTextIncludesVersion(t *testing.T) {
	if !strings.Contains(helpText("v9.9.9"), "v9.9.9") {
		t.Error("help must print the version")
	}
}

func TestHelpTextEmptyVersion(t *testing.T) {
	// A zero-value version (bad ldflags) must not leave a double
	// space in the header; alignment spaces inside the body are fine.
	header := strings.SplitN(helpText(""), "\n", 2)[0]
	if want := "lazycloud — browse GCP resources in the terminal"; header != want {
		t.Errorf("header = %q, want %q", header, want)
	}
}
