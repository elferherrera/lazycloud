package cloud

import (
	"errors"
	"reflect"
	"testing"
)

func TestRegistryHasGCP(t *testing.T) {
	want := []string{"gcp"}
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
	if want := "unknown cloud \"wat\", valid: gcp"; err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
}

func TestGetUnknownFromEnvMessage(t *testing.T) {
	// Execute marks env-sourced errors FromEnv so the message names
	// LAZYCLOUD_CLOUD instead of the flag.
	uc := &ErrUnknownCloud{Name: "azure", Valid: Names(), FromEnv: true}
	want := "invalid LAZYCLOUD_CLOUD \"azure\", valid: gcp"
	if uc.Error() != want {
		t.Fatalf("message = %q, want %q", uc.Error(), want)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("LAZYCLOUD_CLOUD", GCP)
	if got := FromEnv(); got != GCP {
		t.Fatalf("FromEnv() = %q, want %q", got, GCP)
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
