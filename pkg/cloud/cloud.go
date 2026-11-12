package cloud

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

// Cloud names select which provider backs an invocation. The empty
// string means the default (gcp), so callers that never learned about
// clouds keep their old behavior.
const (
	GCP = "gcp"
)

// EnvCloud is the environment variable that fills --cloud when the
// flag is absent.
const EnvCloud = "LAZYCLOUD_CLOUD"

// ErrUnknownCloud reports a cloud name outside the registry. Its
// message carries the valid names, so flag and env errors read the
// same and no caller needs its own valid-name list.
type ErrUnknownCloud struct {
	Name    string
	Valid   []string
	FromEnv bool
}

func (e *ErrUnknownCloud) Error() string {
	if e.FromEnv {
		return fmt.Sprintf("invalid %s %q, valid: %s", EnvCloud, e.Name, strings.Join(e.Valid, " "))
	}
	return fmt.Sprintf("unknown cloud %q, valid: %s", e.Name, strings.Join(e.Valid, " "))
}

// Fetchers is one cloud's read model over the shared resource nouns.
// Every field has the same signature across clouds, so the registry,
// the cache flow, and the CLI treat each cloud identically. A
// provider fills every field its cloud supports.
type Fetchers struct {
	Projects        func() ([]providers.Project, error)
	VMs             func(string) ([]providers.VM, error)
	Disks           func(string) ([]providers.Disk, error)
	Clusters        func(string) ([]providers.Cluster, error)
	Networks        func(string) ([]providers.Network, error)
	Subnetworks     func(string) ([]providers.Subnetwork, error)
	Buckets         func(string) ([]providers.Bucket, error)
	ServiceAccounts func(string) ([]providers.ServiceAccount, error)
	IAMBindings     func(string) ([]providers.IAMBinding, error)
}

// Provider is everything lazycloud needs from one cloud: its name, the
// fetchers behind the shared nouns, and the cache file that stores
// them. Cache and fetchers travel together so an invocation never
// mixes one cloud's cache with another's fetchers.
type Provider struct {
	// Name is the --cloud value and the registry key.
	Name string
	// Fetchers backs the shared resource nouns.
	Fetchers Fetchers
	// TableHeaders overrides the --table columns for a noun. Keys are
	// resource nouns ("projects"); absent means the standard columns,
	// so a cloud only overrides what its data actually reshapes.
	TableHeaders map[string][]string
	// CacheFileSuffix distinguishes the cloud's cache db under
	// ~/.config/lazycloud/. Empty means the original cache.db, so
	// existing GCP users keep their data with no migration.
	CacheFileSuffix string
	// NewCache opens the cloud's cache db.
	NewCache func() (*providers.Cache, error)
}

// Register adds a provider. It panics on an empty name or a duplicate
// registration, because both are programming errors caught at startup,
// not runtime conditions to handle gracefully. The registry is frozen
// once init() ran: runtime Register calls from goroutines would race
// on the map, so registration stays an init-time affair.
func Register(p Provider) {
	if p.Name == "" {
		panic("cloud.Register: empty provider name")
	}
	if _, dup := providersByName[p.Name]; dup {
		panic("cloud.Register: duplicate provider " + p.Name)
	}
	providersByName[p.Name] = p
}

var providersByName = map[string]Provider{}

func init() {
	Register(Provider{
		Name:     GCP,
		Fetchers: gcpFetchers(),
		NewCache: providers.NewCache,
	})
}

// Names returns the registered cloud names sorted, for error messages
// and help text.
func Names() []string {
	names := make([]string, 0, len(providersByName))
	for name := range providersByName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Get resolves a cloud name to its provider. The empty name resolves
// to the default (gcp); any other unregistered name returns
// *ErrUnknownCloud with the valid names inside.
func Get(name string) (Provider, error) {
	if name == "" {
		name = GCP
	}
	p, ok := providersByName[name]
	if !ok {
		return Provider{}, &ErrUnknownCloud{Name: name, Valid: Names()}
	}
	return p, nil
}

// FromEnv reads the LAZYCLOUD_CLOUD default the flag layer applies
// when --cloud is absent. An invalid env value is an *ErrUnknownCloud
// marked FromEnv so the error names LAZYCLOUD_CLOUD.
func FromEnv() string {
	return os.Getenv(EnvCloud)
}

// DefaultCacheDir is where all cache dbs live, exported for
// constructors that place a new cloud's db next to the existing ones.
func DefaultCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lazycloud"), nil
}
