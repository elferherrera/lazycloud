package cloud

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRegistryHasGCPAndNebius(t *testing.T) {
	want := []string{"gcp", "nebius"}
	if got := Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
}

func TestGetDefault(t *testing.T) {
	p, err := Get("")
	if err != nil {
		t.Fatalf("Get(\"\"): %v", err)
	}
	if p.Name != GCP {
		t.Fatalf("default cloud = %q, want gcp", p.Name)
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := Get("wat")
	var uc *ErrUnknownCloud
	if !errors.As(err, &uc) {
		t.Fatalf("Get(wat) = %v, want ErrUnknownCloud", err)
	}
	if uc.Name != "wat" || uc.FromEnv {
		t.Fatalf("error = %+v", uc)
	}
	if want := "unknown cloud \"wat\", valid: gcp nebius"; err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
}

func TestGetUnknownFromEnvMessage(t *testing.T) {
	// Execute marks env-sourced errors FromEnv so the message names
	// LAZYCLOUD_CLOUD instead of the flag.
	uc := &ErrUnknownCloud{Name: "azure", Valid: Names(), FromEnv: true}
	want := "invalid LAZYCLOUD_CLOUD \"azure\", valid: gcp nebius"
	if uc.Error() != want {
		t.Fatalf("message = %q, want %q", uc.Error(), want)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("LAZYCLOUD_CLOUD", Nebius)
	if got := FromEnv(); got != Nebius {
		t.Fatalf("FromEnv() = %q, want %q", got, Nebius)
	}
}

func TestRegisterPanics(t *testing.T) {
	for name, p := range map[string]Provider{
		"empty name": {Name: ""},
		"duplicate":  {Name: GCP},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: Register must panic", name)
				}
			}()
			Register(p)
		}()
	}
}

func TestGCPFetchersWired(t *testing.T) {
	p, err := Get(GCP)
	if err != nil {
		t.Fatalf("Get(gcp): %v", err)
	}
	fields := reflect.ValueOf(p.Fetchers)
	for i := 0; i < fields.NumField(); i++ {
		if fields.Field(i).IsNil() {
			t.Errorf("gcp fetchers field %s is nil", fields.Type().Field(i).Name)
		}
	}
}

func TestNebiusFetchersWired(t *testing.T) {
	p, err := Get(Nebius)
	if err != nil {
		t.Fatalf("Get(nebius): %v", err)
	}
	fields := reflect.ValueOf(p.Fetchers)
	for i := 0; i < fields.NumField(); i++ {
		if fields.Field(i).IsNil() {
			t.Errorf("nebius fetchers field %s is nil", fields.Type().Field(i).Name)
		}
	}
}

func TestNewNebiusCache(t *testing.T) {
	// NewNebiusCache opens the db under the user config dir. HOME is
	// redirected to a temp dir so the test never touches the real
	// ~/.config/lazycloud.
	t.Setenv("HOME", t.TempDir())
	db, err := NewNebiusCache()
	if err != nil {
		t.Fatalf("NewNebiusCache: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "lazycloud", "cache-nebius.db")); err != nil {
		t.Fatalf("cache-nebius.db missing: %v", err)
	}
}
