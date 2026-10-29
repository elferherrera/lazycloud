package providers

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	cache, err := NewCacheAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("NewCacheAt() error = %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	return cache
}

func TestSaveAndListProjects(t *testing.T) {
	cache := newTestCache(t)
	projects := []Project{
		{ProjectID: "beta", Name: "Beta", ProjectNumber: "2"},
		{ProjectID: "alpha", Name: "Alpha", ProjectNumber: "1"},
	}
	if err := cache.SaveProjects(projects); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	got, err := cache.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListProjects() len = %d, want 2", len(got))
	}
	if got[0].ProjectID != "alpha" || got[1].ProjectID != "beta" {
		t.Fatalf("ListProjects() not ordered by project id: %+v", got)
	}
	if got[0].Name != "Alpha" || got[0].ProjectNumber != "1" {
		t.Fatalf("ListProjects()[0] = %+v", got[0])
	}
}

func TestDeleteProject(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.SaveProjects([]Project{
		{ProjectID: "p1", Name: "P One", ProjectNumber: "111"},
		{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"},
	}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if err := cache.SaveIPAddresses("p1", []IPAddress{
		{ProjectID: "p1", Name: "ip-a", Address: "10.0.0.1", AddressType: "INTERNAL", Status: "RESERVED", Region: "europe-west1"},
	}); err != nil {
		t.Fatalf("SaveIPAddresses() error = %v", err)
	}
	if err := cache.SaveNetworks("p1", []Network{{Name: "vpc-a"}}, []Subnetwork{{Name: "sn-a", Region: "europe-west1", Network: "vpc-a"}}); err != nil {
		t.Fatalf("SaveNetworks() error = %v", err)
	}

	p, err := cache.DeleteProject("p1")
	if err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if p == nil || p.ProjectID != "p1" || p.Name != "P One" {
		t.Fatalf("DeleteProject() = %+v, want deleted project", p)
	}

	got, err := cache.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(got) != 1 || got[0].ProjectID != "p2" {
		t.Fatalf("ListProjects() = %+v, want only p2", got)
	}
	ips, err := cache.ListIPAddresses("p1")
	if err != nil {
		t.Fatalf("ListIPAddresses() error = %v", err)
	}
	if len(ips) != 0 {
		t.Fatalf("ListIPAddresses(p1) len = %d, want 0 after delete", len(ips))
	}
	networks, err := cache.ListNetworks("p1")
	if err != nil {
		t.Fatalf("ListNetworks() error = %v", err)
	}
	if len(networks) != 0 {
		t.Fatalf("ListNetworks(p1) len = %d, want 0 after delete", len(networks))
	}

	deleted, err := cache.ListDeletedProjects()
	if err != nil {
		t.Fatalf("ListDeletedProjects() error = %v", err)
	}
	if len(deleted) != 1 || deleted[0].ProjectID != "p1" || deleted[0].Name != "P One" || deleted[0].ProjectNumber != "111" {
		t.Fatalf("ListDeletedProjects() = %+v, want p1 tombstone", deleted)
	}

	p, err = cache.DeleteProject("missing")
	if err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if p != nil {
		t.Fatalf("DeleteProject() = %+v, want nil for missing project", p)
	}

	if err := cache.SaveProjects([]Project{
		{ProjectID: "p1", Name: "P One", ProjectNumber: "111"},
		{ProjectID: "p3", Name: "P Three", ProjectNumber: "333"},
	}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	p, err = cache.DeleteProject("p1")
	if err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if p != nil {
		t.Fatalf("DeleteProject() = %+v, want nil for tombstoned project", p)
	}
	deleted, err = cache.ListDeletedProjects()
	if err != nil {
		t.Fatalf("ListDeletedProjects() error = %v", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("ListDeletedProjects() len = %d, want 1", len(deleted))
	}
}

func TestSaveProjectsSkipsDeleted(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.SaveProjects([]Project{
		{ProjectID: "p1", Name: "P One", ProjectNumber: "111"},
		{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"},
	}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if _, err := cache.DeleteProject("p1"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if err := cache.SaveProjects([]Project{
		{ProjectID: "p1", Name: "P One", ProjectNumber: "111"},
		{ProjectID: "p3", Name: "P Three", ProjectNumber: "333"},
	}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	got, err := cache.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(got) != 1 || got[0].ProjectID != "p3" {
		t.Fatalf("ListProjects() = %+v, want only p3 (deleted p1 skipped)", got)
	}
}

func TestRestoreProject(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.SaveProjects([]Project{
		{ProjectID: "p1", Name: "P One", ProjectNumber: "111"},
		{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"},
		{ProjectID: "p3", Name: "P Three", ProjectNumber: "333"},
	}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	for _, id := range []string{"p1", "p2"} {
		if _, err := cache.DeleteProject(id); err != nil {
			t.Fatalf("DeleteProject(%s) error = %v", id, err)
		}
		time.Sleep(time.Second)
	}

	p, err := cache.RestoreProject()
	if err != nil {
		t.Fatalf("RestoreProject() error = %v", err)
	}
	if p == nil || p.ProjectID != "p2" {
		t.Fatalf("RestoreProject() = %+v, want most recently deleted p2", p)
	}
	got, err := cache.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	found := map[string]bool{}
	for _, proj := range got {
		found[proj.ProjectID] = true
	}
	if !found["p2"] || !found["p3"] {
		t.Fatalf("ListProjects() = %+v, want p2 restored and p3 kept", got)
	}
	if found["p1"] {
		t.Fatal("p1 should still be deleted")
	}
	deleted, err := cache.ListDeletedProjects()
	if err != nil {
		t.Fatalf("ListDeletedProjects() error = %v", err)
	}
	if len(deleted) != 1 || deleted[0].ProjectID != "p1" {
		t.Fatalf("ListDeletedProjects() = %+v, want only p1 tombstone", deleted)
	}

	saved := []Project{{ProjectID: "p1", Name: "P One", ProjectNumber: "111"}, {ProjectID: "p3", Name: "P Three", ProjectNumber: "333"}}
	if err := cache.SaveProjects(saved); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	p, err = cache.RestoreProject()
	if err != nil {
		t.Fatalf("RestoreProject() error = %v", err)
	}
	if p == nil || p.ProjectID != "p1" {
		t.Fatalf("RestoreProject() = %+v, want p1", p)
	}
	if err := cache.SaveProjects(saved); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}

	p, err = cache.RestoreProject()
	if err != nil {
		t.Fatalf("RestoreProject() error = %v", err)
	}
	if p != nil {
		t.Fatalf("RestoreProject() = %+v, want nil when nothing is deleted", p)
	}
}

func TestSaveAndListServiceAccounts(t *testing.T) {
	cache := newTestCache(t)
	accounts := []ServiceAccount{
		{ProjectID: "p1", Email: "sa-b@p1.iam.gserviceaccounts.com", DisplayName: "SA B", UniqueID: "222", Disabled: true},
		{ProjectID: "p1", Email: "sa-a@p1.iam.gserviceaccounts.com", DisplayName: "SA A", UniqueID: "111", Description: "compute default"},
	}
	if err := cache.SaveServiceAccounts("p1", accounts); err != nil {
		t.Fatalf("SaveServiceAccounts() error = %v", err)
	}
	if err := cache.SaveServiceAccounts("p2", accounts[:1]); err != nil {
		t.Fatalf("SaveServiceAccounts() other project error = %v", err)
	}

	got, err := cache.ListServiceAccounts("p1")
	if err != nil {
		t.Fatalf("ListServiceAccounts() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListServiceAccounts() len = %d, want 2", len(got))
	}
	if got[0].Email != "sa-a@p1.iam.gserviceaccounts.com" || got[0].UniqueID != "111" {
		t.Fatalf("ListServiceAccounts() not ordered by email: %+v", got)
	}
	if got[0].Description != "compute default" || got[0].UpdatedAt == "" {
		t.Fatalf("ListServiceAccounts()[0] = %+v", got[0])
	}
	if !got[1].Disabled {
		t.Fatalf("ListServiceAccounts()[1].Disabled = false, want true: %+v", got[1])
	}

	if err := cache.SaveServiceAccounts("p1", accounts[:1]); err != nil {
		t.Fatalf("SaveServiceAccounts() replace error = %v", err)
	}
	got, err = cache.ListServiceAccounts("p1")
	if err != nil {
		t.Fatalf("ListServiceAccounts() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListServiceAccounts() len = %d, want 1 after replace", len(got))
	}
	other, err := cache.ListServiceAccounts("p2")
	if err != nil {
		t.Fatalf("ListServiceAccounts() error = %v", err)
	}
	if len(other) != 1 {
		t.Fatalf("ListServiceAccounts(p2) len = %d, want 1 (other project untouched)", len(other))
	}

	sa, err := cache.GetServiceAccount("p1", "sa-b@p1.iam.gserviceaccounts.com")
	if err != nil {
		t.Fatalf("GetServiceAccount() error = %v", err)
	}
	if sa == nil || !sa.Disabled {
		t.Fatalf("GetServiceAccount() = %+v", sa)
	}
	sa, err = cache.GetServiceAccount("p1", "missing@p1.iam.gserviceaccounts.com")
	if err != nil {
		t.Fatalf("GetServiceAccount() error = %v", err)
	}
	if sa != nil {
		t.Fatalf("GetServiceAccount() = %+v, want nil", sa)
	}

	all, err := cache.ListAllServiceAccounts()
	if err != nil {
		t.Fatalf("ListAllServiceAccounts() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListAllServiceAccounts() len = %d, want 2", len(all))
	}

	if _, err := cache.DeleteProject("p1"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	got, err = cache.ListServiceAccounts("p1")
	if err != nil {
		t.Fatalf("ListServiceAccounts() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListServiceAccounts(p1) len = %d, want 1 (project not in cache, no cascade)", len(got))
	}

	if err := cache.SaveProjects([]Project{{ProjectID: "p1", Name: "P One", ProjectNumber: "111"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	p, err := cache.DeleteProject("p1")
	if err != nil || p == nil {
		t.Fatalf("DeleteProject() = %v, %v, want project", p, err)
	}
	got, err = cache.ListServiceAccounts("p1")
	if err != nil {
		t.Fatalf("ListServiceAccounts() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListServiceAccounts(p1) len = %d, want 0 after project delete", len(got))
	}
}

func TestSaveAndListVMs(t *testing.T) {
	cache := newTestCache(t)
	vms := []VM{
		{
			ProjectID:   "p1",
			Name:        "vm-a",
			InstanceID:  "111222333",
			Zone:        "europe-west1-b",
			MachineType: "n1-standard-1",
			Status:      "RUNNING",
			Labels:      map[string]string{"env": "prod", "team": "payments"},
			NetworkTags: []string{"https-server", "ssh"},
			ServiceAccounts: []VMServiceAccount{
				{Email: "123-compute@developer.gserviceaccount.com", Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"}},
			},
			InstanceTemplate: "mig-template",
			Interfaces: []VMInterface{
				{Network: "vpc-a", Subnetwork: "sn-a", InternalIP: "10.0.0.3", ExternalIP: "34.1.1.1"},
			},
			BootDisk:  &VMDisk{Name: "vm-a", DeviceName: "boot", Mode: "READ_WRITE", AutoDelete: true, SizeGB: 20},
			DataDisks: []VMDisk{{Name: "data-1", DeviceName: "data-1", Mode: "READ_WRITE", SizeGB: 100}},
		},
		{ProjectID: "p1", Name: "vm-a", Zone: "us-central1-a", Status: "TERMINATED"},
	}
	if err := cache.SaveVMs("p1", vms); err != nil {
		t.Fatalf("SaveVMs() error = %v", err)
	}
	if err := cache.SaveVMs("p2", vms[:1]); err != nil {
		t.Fatalf("SaveVMs() other project error = %v", err)
	}

	got, err := cache.ListVMs("p1")
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListVMs() len = %d, want 2 (same name in two zones)", len(got))
	}
	if got[0].Zone != "europe-west1-b" || got[1].Zone != "us-central1-a" {
		t.Fatalf("ListVMs() not ordered by zone, name: %+v", got)
	}
	if got[0].InstanceID != "111222333" {
		t.Fatalf("ListVMs()[0].InstanceID = %q, want 111222333", got[0].InstanceID)
	}
	if len(got[0].Interfaces) != 1 || got[0].Interfaces[0].Subnetwork != "sn-a" || got[0].Interfaces[0].ExternalIP != "34.1.1.1" {
		t.Fatalf("ListVMs()[0].Interfaces = %+v", got[0].Interfaces)
	}
	if len(got[0].Labels) != 2 || got[0].Labels["env"] != "prod" || got[0].Labels["team"] != "payments" {
		t.Fatalf("ListVMs()[0].Labels = %+v", got[0].Labels)
	}
	if len(got[0].NetworkTags) != 2 || got[0].NetworkTags[0] != "https-server" || got[0].NetworkTags[1] != "ssh" {
		t.Fatalf("ListVMs()[0].NetworkTags = %+v", got[0].NetworkTags)
	}
	if len(got[0].ServiceAccounts) != 1 || got[0].ServiceAccounts[0].Email != "123-compute@developer.gserviceaccount.com" ||
		len(got[0].ServiceAccounts[0].Scopes) != 1 {
		t.Fatalf("ListVMs()[0].ServiceAccounts = %+v", got[0].ServiceAccounts)
	}
	if got[0].InstanceTemplate != "mig-template" {
		t.Fatalf("ListVMs()[0].InstanceTemplate = %q, want mig-template", got[0].InstanceTemplate)
	}
	if got[0].BootDisk == nil || got[0].BootDisk.Name != "vm-a" || !got[0].BootDisk.AutoDelete {
		t.Fatalf("ListVMs()[0].BootDisk = %+v", got[0].BootDisk)
	}
	if len(got[0].DataDisks) != 1 || got[0].DataDisks[0].SizeGB != 100 {
		t.Fatalf("ListVMs()[0].DataDisks = %+v", got[0].DataDisks)
	}
	if got[0].UpdatedAt == "" {
		t.Fatal("ListVMs()[0].UpdatedAt empty, want timestamp")
	}
	if got[1].BootDisk != nil || got[1].DataDisks != nil || got[1].Interfaces != nil || got[1].Labels != nil ||
		got[1].NetworkTags != nil || got[1].ServiceAccounts != nil || got[1].InstanceTemplate != "" {
		t.Fatalf("ListVMs()[1] nested fields = %+v, want nils", got[1])
	}

	all, err := cache.ListAllVMs()
	if err != nil {
		t.Fatalf("ListAllVMs() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAllVMs() len = %d, want 3", len(all))
	}

	vm, err := cache.GetVM("p1", "us-central1-a", "vm-a")
	if err != nil {
		t.Fatalf("GetVM() error = %v", err)
	}
	if vm == nil || vm.Status != "TERMINATED" {
		t.Fatalf("GetVM() = %+v", vm)
	}
	vm, err = cache.GetVM("p1", "europe-west1-c", "missing")
	if err != nil {
		t.Fatalf("GetVM() error = %v", err)
	}
	if vm != nil {
		t.Fatalf("GetVM() = %+v, want nil", vm)
	}

	if err := cache.SaveVMs("p1", vms[:1]); err != nil {
		t.Fatalf("SaveVMs() replace error = %v", err)
	}
	got, err = cache.ListVMs("p1")
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListVMs() len = %d, want 1 after replace", len(got))
	}

	if err := cache.SaveProjects([]Project{{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if _, err := cache.DeleteProject("p2"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	other, err := cache.ListVMs("p2")
	if err != nil {
		t.Fatalf("ListVMs() error = %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("ListVMs(p2) len = %d, want 0 after project delete", len(other))
	}
}

func TestSaveAndListDisks(t *testing.T) {
	cache := newTestCache(t)
	disks := []Disk{
		{ProjectID: "p1", Name: "disk-b", Zone: "europe-west1-b", SizeGB: 100, Type: "pd-balanced", Status: "READY", Users: []string{"vm-a", "vm-b"}},
		{ProjectID: "p1", Name: "disk-a", Zone: "europe-west1-b", SizeGB: 20, Type: "pd-ssd", Status: "READY", SourceImage: "debian-12"},
		{ProjectID: "p1", Name: "disk-a", Zone: "us-central1-a", SizeGB: 50, Type: "pd-standard", Status: "READY"},
	}
	if err := cache.SaveDisks("p1", disks); err != nil {
		t.Fatalf("SaveDisks() error = %v", err)
	}
	if err := cache.SaveDisks("p2", disks[:1]); err != nil {
		t.Fatalf("SaveDisks() other project error = %v", err)
	}

	got, err := cache.ListDisks("p1")
	if err != nil {
		t.Fatalf("ListDisks() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListDisks() len = %d, want 3 (same name in two zones)", len(got))
	}
	if got[0].Name != "disk-a" || got[0].Zone != "europe-west1-b" || got[0].SourceImage != "debian-12" {
		t.Fatalf("ListDisks() not ordered by zone, name: %+v", got)
	}
	if len(got[1].Users) != 2 || got[1].Users[0] != "vm-a" {
		t.Fatalf("ListDisks()[1].Users = %v", got[1].Users)
	}
	if got[1].UpdatedAt == "" {
		t.Fatal("ListDisks()[1].UpdatedAt empty, want timestamp")
	}

	all, err := cache.ListAllDisks()
	if err != nil {
		t.Fatalf("ListAllDisks() error = %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("ListAllDisks() len = %d, want 4", len(all))
	}

	d, err := cache.GetDisk("p1", "us-central1-a", "disk-a")
	if err != nil {
		t.Fatalf("GetDisk() error = %v", err)
	}
	if d == nil || d.SizeGB != 50 {
		t.Fatalf("GetDisk() = %+v", d)
	}
	d, err = cache.GetDisk("p1", "europe-west1-c", "missing")
	if err != nil {
		t.Fatalf("GetDisk() error = %v", err)
	}
	if d != nil {
		t.Fatalf("GetDisk() = %+v, want nil", d)
	}

	if err := cache.SaveDisks("p1", disks[:1]); err != nil {
		t.Fatalf("SaveDisks() replace error = %v", err)
	}
	got, err = cache.ListDisks("p1")
	if err != nil {
		t.Fatalf("ListDisks() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListDisks() len = %d, want 1 after replace", len(got))
	}
	other, err := cache.ListDisks("p2")
	if err != nil {
		t.Fatalf("ListDisks() error = %v", err)
	}
	if len(other) != 1 {
		t.Fatalf("ListDisks(p2) len = %d, want 1 (other project untouched)", len(other))
	}

	if err := cache.SaveProjects([]Project{{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if _, err := cache.DeleteProject("p2"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	other, err = cache.ListDisks("p2")
	if err != nil {
		t.Fatalf("ListDisks() error = %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("ListDisks(p2) len = %d, want 0 after project delete", len(other))
	}
}

func TestSaveAndListBuckets(t *testing.T) {
	cache := newTestCache(t)
	buckets := []Bucket{
		{ProjectID: "p1", Name: "bucket-b", Location: "US", StorageClass: "NEARLINE", UniformAccess: true},
		{ProjectID: "p1", Name: "bucket-a", Location: "EUROPE-WEST1", StorageClass: "STANDARD", Versioning: true, CreatedAt: "2026-01-01T00:00:00Z"},
	}
	if err := cache.SaveBuckets("p1", buckets); err != nil {
		t.Fatalf("SaveBuckets() error = %v", err)
	}
	if err := cache.SaveBuckets("p2", buckets[:1]); err != nil {
		t.Fatalf("SaveBuckets() other project error = %v", err)
	}

	got, err := cache.ListBuckets("p1")
	if err != nil {
		t.Fatalf("ListBuckets() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListBuckets() len = %d, want 2", len(got))
	}
	if got[0].Name != "bucket-a" || got[1].Name != "bucket-b" {
		t.Fatalf("ListBuckets() not ordered by name: %+v", got)
	}
	if !got[0].Versioning || got[0].CreatedAt != "2026-01-01T00:00:00Z" || got[0].UpdatedAt == "" {
		t.Fatalf("ListBuckets()[0] = %+v", got[0])
	}
	if !got[1].UniformAccess || got[1].Versioning {
		t.Fatalf("ListBuckets()[1] = %+v", got[1])
	}

	all, err := cache.ListAllBuckets()
	if err != nil {
		t.Fatalf("ListAllBuckets() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAllBuckets() len = %d, want 3", len(all))
	}

	b, err := cache.GetBucket("p1", "bucket-b")
	if err != nil {
		t.Fatalf("GetBucket() error = %v", err)
	}
	if b == nil || !b.UniformAccess {
		t.Fatalf("GetBucket() = %+v", b)
	}
	b, err = cache.GetBucket("p1", "missing")
	if err != nil {
		t.Fatalf("GetBucket() error = %v", err)
	}
	if b != nil {
		t.Fatalf("GetBucket() = %+v, want nil", b)
	}

	if err := cache.SaveBuckets("p1", buckets[:1]); err != nil {
		t.Fatalf("SaveBuckets() replace error = %v", err)
	}
	got, err = cache.ListBuckets("p1")
	if err != nil {
		t.Fatalf("ListBuckets() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListBuckets() len = %d, want 1 after replace", len(got))
	}
	other, err := cache.ListBuckets("p2")
	if err != nil {
		t.Fatalf("ListBuckets() error = %v", err)
	}
	if len(other) != 1 {
		t.Fatalf("ListBuckets(p2) len = %d, want 1 (other project untouched)", len(other))
	}

	if err := cache.SaveProjects([]Project{{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if _, err := cache.DeleteProject("p2"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	other, err = cache.ListBuckets("p2")
	if err != nil {
		t.Fatalf("ListBuckets() error = %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("ListBuckets(p2) len = %d, want 0 after project delete", len(other))
	}
}

func TestSaveAndListLoadBalancerParts(t *testing.T) {
	cache := newTestCache(t)
	rules := []ForwardingRule{
		{ProjectID: "p1", Name: "fr-web", Region: "global", IPAddress: "34.1.1.1", PortRange: "443", Protocol: "TCP", Scheme: "EXTERNAL", TargetKind: "targetHttpsProxies", Target: "proxy-a"},
		{ProjectID: "p1", Name: "fr-int", Region: "europe-west1", IPAddress: "10.0.0.5", Protocol: "TCP", Scheme: "INTERNAL", TargetKind: "backendServices", Target: "svc-int"},
	}
	proxies := []TargetProxy{
		{ProjectID: "p1", Kind: "targetHttpsProxies", Name: "proxy-a", URLMap: "map-a"},
		{ProjectID: "p1", Kind: "targetTcpProxies", Name: "proxy-b", BackendService: "svc-tcp"},
	}
	urlMaps := []URLMap{
		{ProjectID: "p1", Name: "map-a", DefaultService: "svc-web", PathRules: []PathRule{{Paths: []string{"/api"}, Service: "svc-api"}}},
	}
	services := []BackendService{
		{ProjectID: "p1", Name: "svc-web", Region: "global", Protocol: "HTTPS", PortName: "https", Backends: []Backend{{Group: "ig-a", BalancingMode: "UTILIZATION"}}, HealthChecks: []string{"hc-a"}},
		{ProjectID: "p1", Name: "svc-api", Region: "global", Protocol: "HTTPS", PortName: "https", Backends: []Backend{{Group: "neg-api"}}},
		{ProjectID: "p1", Name: "svc-int", Region: "europe-west1", Protocol: "TCP"},
	}
	if err := cache.SaveForwardingRules("p1", rules); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	if err := cache.SaveForwardingRules("p2", rules[:1]); err != nil {
		t.Fatalf("SaveForwardingRules() other project error = %v", err)
	}
	if err := cache.SaveTargetProxies("p1", proxies); err != nil {
		t.Fatalf("SaveTargetProxies() error = %v", err)
	}
	if err := cache.SaveURLMaps("p1", urlMaps); err != nil {
		t.Fatalf("SaveURLMaps() error = %v", err)
	}
	if err := cache.SaveBackendServices("p1", services); err != nil {
		t.Fatalf("SaveBackendServices() error = %v", err)
	}

	gotRules, err := cache.ListForwardingRules("p1")
	if err != nil {
		t.Fatalf("ListForwardingRules() error = %v", err)
	}
	if len(gotRules) != 2 {
		t.Fatalf("ListForwardingRules() len = %d, want 2", len(gotRules))
	}
	if gotRules[0].Region != "europe-west1" || gotRules[0].Name != "fr-int" {
		t.Fatalf("ListForwardingRules() not ordered by region, name: %+v", gotRules)
	}
	if gotRules[1].TargetKind != "targetHttpsProxies" || gotRules[1].Target != "proxy-a" || gotRules[1].UpdatedAt == "" {
		t.Fatalf("ListForwardingRules()[1] = %+v", gotRules[1])
	}

	gotProxies, err := cache.ListTargetProxies("p1")
	if err != nil {
		t.Fatalf("ListTargetProxies() error = %v", err)
	}
	if len(gotProxies) != 2 {
		t.Fatalf("ListTargetProxies() len = %d, want 2", len(gotProxies))
	}
	if gotProxies[0].Name != "proxy-a" || gotProxies[0].URLMap != "map-a" || gotProxies[1].BackendService != "svc-tcp" {
		t.Fatalf("ListTargetProxies() = %+v", gotProxies)
	}

	gotMaps, err := cache.ListURLMaps("p1")
	if err != nil {
		t.Fatalf("ListURLMaps() error = %v", err)
	}
	if len(gotMaps) != 1 {
		t.Fatalf("ListURLMaps() len = %d, want 1", len(gotMaps))
	}
	if len(gotMaps[0].PathRules) != 1 || gotMaps[0].PathRules[0].Service != "svc-api" || gotMaps[0].PathRules[0].Paths[0] != "/api" {
		t.Fatalf("ListURLMaps()[0].PathRules = %+v", gotMaps[0].PathRules)
	}

	gotServices, err := cache.ListBackendServices("p1")
	if err != nil {
		t.Fatalf("ListBackendServices() error = %v", err)
	}
	if len(gotServices) != 3 {
		t.Fatalf("ListBackendServices() len = %d, want 3", len(gotServices))
	}
	if gotServices[0].Region != "europe-west1" || gotServices[0].Name != "svc-int" {
		t.Fatalf("ListBackendServices() not ordered by region, name: %+v", gotServices)
	}
	if len(gotServices[2].Backends) != 1 || gotServices[2].Backends[0].Group != "ig-a" || gotServices[2].Backends[0].BalancingMode != "UTILIZATION" {
		t.Fatalf("ListBackendServices()[2].Backends = %+v", gotServices[2].Backends)
	}
	if len(gotServices[2].HealthChecks) != 1 || gotServices[2].HealthChecks[0] != "hc-a" {
		t.Fatalf("ListBackendServices()[2].HealthChecks = %+v", gotServices[2].HealthChecks)
	}

	allRules, err := cache.ListAllForwardingRules()
	if err != nil {
		t.Fatalf("ListAllForwardingRules() error = %v", err)
	}
	if len(allRules) != 3 || allRules[0].ProjectID != "p1" {
		t.Fatalf("ListAllForwardingRules() = %+v, want 3 ordered by project", allRules)
	}
	allProxies, err := cache.ListAllTargetProxies()
	if err != nil {
		t.Fatalf("ListAllTargetProxies() error = %v", err)
	}
	if len(allProxies) != 2 {
		t.Fatalf("ListAllTargetProxies() len = %d, want 2", len(allProxies))
	}
	allMaps, err := cache.ListAllURLMaps()
	if err != nil {
		t.Fatalf("ListAllURLMaps() error = %v", err)
	}
	if len(allMaps) != 1 {
		t.Fatalf("ListAllURLMaps() len = %d, want 1", len(allMaps))
	}
	allServices, err := cache.ListAllBackendServices()
	if err != nil {
		t.Fatalf("ListAllBackendServices() error = %v", err)
	}
	if len(allServices) != 3 {
		t.Fatalf("ListAllBackendServices() len = %d, want 3", len(allServices))
	}

	rule, err := cache.GetForwardingRule("p1", "global", "fr-web")
	if err != nil {
		t.Fatalf("GetForwardingRule() error = %v", err)
	}
	if rule == nil || rule.IPAddress != "34.1.1.1" {
		t.Fatalf("GetForwardingRule() = %+v", rule)
	}
	proxy, err := cache.GetTargetProxy("p1", "proxy-b")
	if err != nil {
		t.Fatalf("GetTargetProxy() error = %v", err)
	}
	if proxy == nil || proxy.Kind != "targetTcpProxies" {
		t.Fatalf("GetTargetProxy() = %+v", proxy)
	}
	urlMap, err := cache.GetURLMap("p1", "map-a")
	if err != nil {
		t.Fatalf("GetURLMap() error = %v", err)
	}
	if urlMap == nil || urlMap.DefaultService != "svc-web" {
		t.Fatalf("GetURLMap() = %+v", urlMap)
	}
	svc, err := cache.GetBackendService("p1", "global", "svc-web")
	if err != nil {
		t.Fatalf("GetBackendService() error = %v", err)
	}
	if svc == nil || svc.Protocol != "HTTPS" {
		t.Fatalf("GetBackendService() = %+v", svc)
	}
	rule, err = cache.GetForwardingRule("p1", "global", "missing")
	if err != nil {
		t.Fatalf("GetForwardingRule() error = %v", err)
	}
	if rule != nil {
		t.Fatalf("GetForwardingRule() = %+v, want nil", rule)
	}

	if err := cache.SaveForwardingRules("p1", rules[:1]); err != nil {
		t.Fatalf("SaveForwardingRules() replace error = %v", err)
	}
	gotRules, err = cache.ListForwardingRules("p1")
	if err != nil {
		t.Fatalf("ListForwardingRules() error = %v", err)
	}
	if len(gotRules) != 1 {
		t.Fatalf("ListForwardingRules() len = %d, want 1 after replace", len(gotRules))
	}

	if err := cache.SaveProjects([]Project{{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if _, err := cache.DeleteProject("p2"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	other, err := cache.ListForwardingRules("p2")
	if err != nil {
		t.Fatalf("ListForwardingRules() error = %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("ListForwardingRules(p2) len = %d, want 0 after project delete", len(other))
	}
}

func TestResolveLoadBalancer(t *testing.T) {
	cache := newTestCache(t)
	rules := []ForwardingRule{
		{ProjectID: "p1", Name: "fr-web", Region: "global", IPAddress: "34.1.1.1", PortRange: "443", Protocol: "TCP", Scheme: "EXTERNAL", TargetKind: "targetHttpsProxies", Target: "proxy-a"},
		{ProjectID: "p1", Name: "fr-int", Region: "europe-west1", IPAddress: "10.0.0.5", Protocol: "TCP", Scheme: "INTERNAL", TargetKind: "backendServices", Target: "svc-int"},
		{ProjectID: "p1", Name: "fr-tcp", Region: "global", IPAddress: "34.1.1.2", PortRange: "80", Protocol: "TCP", Scheme: "EXTERNAL", TargetKind: "targetTcpProxies", Target: "proxy-b"},
	}
	proxies := []TargetProxy{
		{ProjectID: "p1", Kind: "targetHttpsProxies", Name: "proxy-a", URLMap: "map-a"},
		{ProjectID: "p1", Kind: "targetTcpProxies", Name: "proxy-b", BackendService: "svc-tcp"},
	}
	urlMaps := []URLMap{
		{ProjectID: "p1", Name: "map-a", DefaultService: "svc-web", PathRules: []PathRule{{Paths: []string{"/api"}, Service: "svc-api"}, {Paths: []string{"/api"}, Service: "svc-web"}}},
	}
	services := []BackendService{
		{ProjectID: "p1", Name: "svc-web", Region: "global", Protocol: "HTTPS"},
		{ProjectID: "p1", Name: "svc-api", Region: "global", Protocol: "HTTPS"},
		{ProjectID: "p1", Name: "svc-int", Region: "europe-west1", Protocol: "TCP"},
		{ProjectID: "p1", Name: "svc-tcp", Region: "global", Protocol: "TCP"},
	}
	if err := cache.SaveForwardingRules("p1", rules); err != nil {
		t.Fatalf("SaveForwardingRules() error = %v", err)
	}
	if err := cache.SaveTargetProxies("p1", proxies); err != nil {
		t.Fatalf("SaveTargetProxies() error = %v", err)
	}
	if err := cache.SaveURLMaps("p1", urlMaps); err != nil {
		t.Fatalf("SaveURLMaps() error = %v", err)
	}
	if err := cache.SaveBackendServices("p1", services); err != nil {
		t.Fatalf("SaveBackendServices() error = %v", err)
	}

	lb, err := cache.ResolveLoadBalancer("p1", "global", "fr-web")
	if err != nil {
		t.Fatalf("ResolveLoadBalancer() error = %v", err)
	}
	if lb == nil || lb.Rule.Name != "fr-web" {
		t.Fatalf("ResolveLoadBalancer() = %+v", lb)
	}
	if lb.Proxy == nil || lb.Proxy.Name != "proxy-a" {
		t.Fatalf("ResolveLoadBalancer() proxy = %+v", lb.Proxy)
	}
	if lb.URLMap == nil || lb.URLMap.Name != "map-a" {
		t.Fatalf("ResolveLoadBalancer() url map = %+v", lb.URLMap)
	}
	if len(lb.BackendServices) != 2 || lb.BackendServices[0].Name != "svc-web" || lb.BackendServices[1].Name != "svc-api" {
		t.Fatalf("ResolveLoadBalancer() backend services = %+v, want svc-web then svc-api (deduped)", lb.BackendServices)
	}

	lb, err = cache.ResolveLoadBalancer("p1", "europe-west1", "fr-int")
	if err != nil {
		t.Fatalf("ResolveLoadBalancer() error = %v", err)
	}
	if lb == nil {
		t.Fatal("ResolveLoadBalancer() = nil, want internal load balancer")
	}
	if lb.Proxy != nil || lb.URLMap != nil {
		t.Fatalf("ResolveLoadBalancer() internal lb should have no proxy or url map: %+v", lb)
	}
	if len(lb.BackendServices) != 1 || lb.BackendServices[0].Name != "svc-int" {
		t.Fatalf("ResolveLoadBalancer() internal backend services = %+v", lb.BackendServices)
	}

	lb, err = cache.ResolveLoadBalancer("p1", "global", "fr-tcp")
	if err != nil {
		t.Fatalf("ResolveLoadBalancer() error = %v", err)
	}
	if lb == nil || lb.Proxy == nil || len(lb.BackendServices) != 1 || lb.BackendServices[0].Name != "svc-tcp" {
		t.Fatalf("ResolveLoadBalancer() tcp proxy chain = %+v", lb)
	}

	lb, err = cache.ResolveLoadBalancer("p1", "global", "missing")
	if err != nil {
		t.Fatalf("ResolveLoadBalancer() error = %v", err)
	}
	if lb != nil {
		t.Fatalf("ResolveLoadBalancer() = %+v, want nil for missing rule", lb)
	}
}

func TestSaveAndListClusters(t *testing.T) {
	cache := newTestCache(t)
	clusters := []Cluster{
		{
			ProjectID:   "p1",
			Name:        "cluster-b",
			Location:    "europe-west1",
			Status:      "RUNNING",
			Version:     "1.31.1-gke.1",
			Endpoint:    "34.1.1.1",
			NodePools:   []ClusterNodePool{{Name: "pool-a", Version: "1.31.1-gke.1", NodeCount: 3}, {Name: "pool-b", Version: "1.31.1-gke.1", NodeCount: 2}},
			CreatedAt:   "2026-01-01T00:00:00Z",
			Description: "second",
		},
		{ProjectID: "p1", Name: "cluster-a", Location: "europe-west1-b", Status: "PROVISIONING", Version: "1.31.1-gke.1"},
	}
	if err := cache.SaveClusters("p1", clusters); err != nil {
		t.Fatalf("SaveClusters() error = %v", err)
	}
	if err := cache.SaveClusters("p2", clusters[:1]); err != nil {
		t.Fatalf("SaveClusters() other project error = %v", err)
	}

	got, err := cache.ListClusters("p1")
	if err != nil {
		t.Fatalf("ListClusters() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListClusters() len = %d, want 2", len(got))
	}
	if got[0].Name != "cluster-a" || got[1].Name != "cluster-b" {
		t.Fatalf("ListClusters() not ordered by name: %+v", got)
	}
	if len(got[1].NodePools) != 2 || got[1].NodePools[0].Name != "pool-a" || got[1].NodePools[0].NodeCount != 3 {
		t.Fatalf("ListClusters()[1].NodePools = %+v", got[1].NodePools)
	}
	if got[1].Endpoint != "34.1.1.1" || got[1].CreatedAt != "2026-01-01T00:00:00Z" || got[1].UpdatedAt == "" {
		t.Fatalf("ListClusters()[1] = %+v", got[1])
	}
	if len(got[0].NodePools) != 0 {
		t.Fatalf("ListClusters()[0].NodePools = %+v, want nils", got[0].NodePools)
	}

	all, err := cache.ListAllClusters()
	if err != nil {
		t.Fatalf("ListAllClusters() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAllClusters() len = %d, want 3", len(all))
	}

	cl, err := cache.GetCluster("p1", "cluster-b")
	if err != nil {
		t.Fatalf("GetCluster() error = %v", err)
	}
	if cl == nil || cl.Status != "RUNNING" {
		t.Fatalf("GetCluster() = %+v", cl)
	}
	cl, err = cache.GetCluster("p1", "missing")
	if err != nil {
		t.Fatalf("GetCluster() error = %v", err)
	}
	if cl != nil {
		t.Fatalf("GetCluster() = %+v, want nil", cl)
	}

	if err := cache.SaveClusters("p1", clusters[:1]); err != nil {
		t.Fatalf("SaveClusters() replace error = %v", err)
	}
	got, err = cache.ListClusters("p1")
	if err != nil {
		t.Fatalf("ListClusters() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListClusters() len = %d, want 1 after replace", len(got))
	}
	other, err := cache.ListClusters("p2")
	if err != nil {
		t.Fatalf("ListClusters() error = %v", err)
	}
	if len(other) != 1 {
		t.Fatalf("ListClusters(p2) len = %d, want 1 (other project untouched)", len(other))
	}

	if err := cache.SaveProjects([]Project{{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if _, err := cache.DeleteProject("p2"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	other, err = cache.ListClusters("p2")
	if err != nil {
		t.Fatalf("ListClusters() error = %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("ListClusters(p2) len = %d, want 0 after project delete", len(other))
	}
}

func TestSaveAndListWorkloads(t *testing.T) {
	cache := newTestCache(t)
	web := []Workload{
		{ProjectID: "p1", Cluster: "cluster-a", Kind: "Deployment", Namespace: "default", Name: "web", Replicas: "3/3", Images: []string{"nginx:1.27"}},
		{ProjectID: "p1", Cluster: "cluster-a", Kind: "DaemonSet", Namespace: "kube-system", Name: "fluentbit", Replicas: "2/2"},
		{ProjectID: "p1", Cluster: "cluster-b", Kind: "StatefulSet", Namespace: "db", Name: "postgres", Replicas: "1/2", Images: []string{"postgres:16"}},
	}
	if err := cache.SaveWorkloads("p1", "cluster-a", web[:2]); err != nil {
		t.Fatalf("SaveWorkloads() error = %v", err)
	}
	if err := cache.SaveWorkloads("p1", "cluster-b", web[2:]); err != nil {
		t.Fatalf("SaveWorkloads() other cluster error = %v", err)
	}
	if err := cache.SaveWorkloads("p2", "cluster-a", web[:1]); err != nil {
		t.Fatalf("SaveWorkloads() other project error = %v", err)
	}

	got, err := cache.ListClusterWorkloads("p1", "cluster-a")
	if err != nil {
		t.Fatalf("ListClusterWorkloads() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListClusterWorkloads() len = %d, want 2", len(got))
	}
	if got[0].Cluster != "cluster-a" || got[0].Namespace != "default" {
		t.Fatalf("ListClusterWorkloads() not ordered by namespace, kind: %+v", got)
	}
	if got[0].Kind != "Deployment" || got[0].Replicas != "3/3" || len(got[0].Images) != 1 || got[0].Images[0] != "nginx:1.27" || got[0].UpdatedAt == "" {
		t.Fatalf("ListClusterWorkloads()[0] = %+v", got[0])
	}
	if len(got[1].Images) != 0 {
		t.Fatalf("ListClusterWorkloads()[1].Images = %+v, want empty", got[1].Images)
	}

	other, err := cache.ListClusterWorkloads("p2", "cluster-a")
	if err != nil {
		t.Fatalf("ListClusterWorkloads() error = %v", err)
	}
	if len(other) != 1 || other[0].ProjectID != "p2" {
		t.Fatalf("ListClusterWorkloads(p2) = %+v, want 1 for other project", other)
	}

	w, err := cache.GetWorkload("p1", "cluster-b", "StatefulSet", "db", "postgres")
	if err != nil {
		t.Fatalf("GetWorkload() error = %v", err)
	}
	if w == nil || w.Replicas != "1/2" {
		t.Fatalf("GetWorkload() = %+v", w)
	}
	w, err = cache.GetWorkload("p1", "cluster-b", "Deployment", "default", "missing")
	if err != nil {
		t.Fatalf("GetWorkload() error = %v", err)
	}
	if w != nil {
		t.Fatalf("GetWorkload() = %+v, want nil", w)
	}

	if err := cache.SaveWorkloads("p1", "cluster-a", web[:1]); err != nil {
		t.Fatalf("SaveWorkloads() replace error = %v", err)
	}
	got, err = cache.ListClusterWorkloads("p1", "cluster-a")
	if err != nil {
		t.Fatalf("ListClusterWorkloads() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListClusterWorkloads() len = %d, want 1 after replace", len(got))
	}
	other, err = cache.ListClusterWorkloads("p1", "cluster-b")
	if err != nil {
		t.Fatalf("ListClusterWorkloads() error = %v", err)
	}
	if len(other) != 1 {
		t.Fatalf("ListClusterWorkloads(p1, cluster-b) len = %d, want 1 (other cluster untouched)", len(other))
	}

	if err := cache.SaveProjects([]Project{{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if _, err := cache.DeleteProject("p2"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	other, err = cache.ListClusterWorkloads("p2", "cluster-a")
	if err != nil {
		t.Fatalf("ListClusterWorkloads() error = %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("ListClusterWorkloads(p2) len = %d, want 0 after project delete", len(other))
	}
}

func TestSaveAndListClusterNodes(t *testing.T) {
	cache := newTestCache(t)
	nodes := []ClusterNode{
		{ProjectID: "p1", Cluster: "cluster-a", Name: "gke-cluster-a-pool-a-aaa", NodePool: "pool-a", Status: "Ready", InternalIP: "10.0.0.1"},
		{ProjectID: "p1", Cluster: "cluster-a", Name: "gke-cluster-a-pool-b-bbb", NodePool: "pool-b", Status: "NotReady"},
		{ProjectID: "p1", Cluster: "cluster-b", Name: "gke-cluster-b-pool-a-ccc", NodePool: "pool-a", Status: "Ready"},
	}
	if err := cache.SaveClusterNodes("p1", "cluster-a", nodes[:2]); err != nil {
		t.Fatalf("SaveClusterNodes() error = %v", err)
	}
	if err := cache.SaveClusterNodes("p1", "cluster-b", nodes[2:]); err != nil {
		t.Fatalf("SaveClusterNodes() other cluster error = %v", err)
	}
	if err := cache.SaveClusterNodes("p2", "cluster-a", nodes[:1]); err != nil {
		t.Fatalf("SaveClusterNodes() other project error = %v", err)
	}

	got, err := cache.ListClusterNodes("p1", "cluster-a")
	if err != nil {
		t.Fatalf("ListClusterNodes() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListClusterNodes() len = %d, want 2", len(got))
	}
	if got[0].NodePool != "pool-a" || got[0].Name != "gke-cluster-a-pool-a-aaa" {
		t.Fatalf("ListClusterNodes() not ordered by pool, name: %+v", got)
	}
	if got[0].Status != "Ready" || got[0].InternalIP != "10.0.0.1" || got[0].UpdatedAt == "" {
		t.Fatalf("ListClusterNodes()[0] = %+v", got[0])
	}
	if got[1].InternalIP != "" {
		t.Fatalf("ListClusterNodes()[1].InternalIP = %q, want empty", got[1].InternalIP)
	}

	other, err := cache.ListClusterNodes("p2", "cluster-a")
	if err != nil {
		t.Fatalf("ListClusterNodes() error = %v", err)
	}
	if len(other) != 1 || other[0].ProjectID != "p2" {
		t.Fatalf("ListClusterNodes(p2) = %+v, want 1 for other project", other)
	}

	if err := cache.SaveClusterNodes("p1", "cluster-a", nodes[:1]); err != nil {
		t.Fatalf("SaveClusterNodes() replace error = %v", err)
	}
	got, err = cache.ListClusterNodes("p1", "cluster-a")
	if err != nil {
		t.Fatalf("ListClusterNodes() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListClusterNodes() len = %d, want 1 after replace", len(got))
	}
	other, err = cache.ListClusterNodes("p1", "cluster-b")
	if err != nil {
		t.Fatalf("ListClusterNodes() error = %v", err)
	}
	if len(other) != 1 {
		t.Fatalf("ListClusterNodes(p1, cluster-b) len = %d, want 1 (other cluster untouched)", len(other))
	}

	if err := cache.SaveProjects([]Project{{ProjectID: "p2", Name: "P Two", ProjectNumber: "222"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	if _, err := cache.DeleteProject("p2"); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	other, err = cache.ListClusterNodes("p2", "cluster-a")
	if err != nil {
		t.Fatalf("ListClusterNodes() error = %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("ListClusterNodes(p2) len = %d, want 0 after project delete", len(other))
	}
}

func TestGetProject(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.SaveProjects([]Project{{ProjectID: "p1", Name: "P One", ProjectNumber: "111"}}); err != nil {
		t.Fatalf("SaveProjects() error = %v", err)
	}
	p, err := cache.GetProject("p1")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if p == nil {
		t.Fatal("GetProject() = nil, want project")
	}
	if p.Name != "P One" || p.ProjectNumber != "111" {
		t.Fatalf("GetProject() = %+v", p)
	}
	p, err = cache.GetProject("missing")
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if p != nil {
		t.Fatalf("GetProject() = %+v, want nil", p)
	}
}

func TestSaveIPAddressesUpserts(t *testing.T) {
	cache := newTestCache(t)
	addresses := []IPAddress{
		{ProjectID: "p1", Name: "ip-a", Address: "10.0.0.1", AddressType: "INTERNAL", Status: "RESERVED", Region: "europe-west1", Subnetwork: "subnet-1"},
		{ProjectID: "p1", Name: "ip-b", Address: "34.1.1.1", AddressType: "EXTERNAL", Status: "RESERVED", Region: "global"},
	}
	if err := cache.SaveIPAddresses("p1", addresses); err != nil {
		t.Fatalf("SaveIPAddresses() error = %v", err)
	}
	if err := cache.SaveIPAddresses("p1", addresses); err != nil {
		t.Fatalf("second SaveIPAddresses() error = %v", err)
	}
	got, err := cache.ListIPAddresses("p1")
	if err != nil {
		t.Fatalf("ListIPAddresses() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListIPAddresses() len = %d, want 2 (no duplicates from upsert)", len(got))
	}

	updated := []IPAddress{
		{ProjectID: "p1", Name: "ip-a", Address: "10.0.0.1", AddressType: "INTERNAL", Status: "IN_USE", Region: "europe-west1", Subnetwork: "subnet-1"},
	}
	if err := cache.SaveIPAddresses("p1", updated); err != nil {
		t.Fatalf("SaveIPAddresses() update error = %v", err)
	}
	got, err = cache.ListIPAddresses("p1")
	if err != nil {
		t.Fatalf("ListIPAddresses() error = %v", err)
	}
	var ipA *IPAddress
	for i := range got {
		if got[i].Name == "ip-a" {
			ipA = &got[i]
		}
	}
	if ipA == nil {
		t.Fatal("ip-a missing after update")
	}
	if ipA.Status != "IN_USE" {
		t.Fatalf("ip-a Status = %q, want IN_USE", ipA.Status)
	}
	if ipA.UpdatedAt == "" {
		t.Fatal("ip-a UpdatedAt empty, want timestamp")
	}
	var found bool
	for _, a := range got {
		if a.Name == "ip-b" {
			found = true
		}
	}
	if !found {
		t.Fatal("ip-b was deleted, want upsert to keep unrelated rows")
	}
}

func TestGetIPAddresses(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.SaveIPAddresses("p1", []IPAddress{
		{ProjectID: "p1", Name: "ip-a", Address: "10.0.0.1", AddressType: "INTERNAL", Status: "RESERVED", Region: "europe-west1", Subnetwork: "subnet-a"},
		{ProjectID: "p1", Name: "ip-a", Address: "10.0.0.1", AddressType: "INTERNAL", Status: "RESERVED", Region: "europe-west1", Subnetwork: "subnet-b"},
		{ProjectID: "p1", Name: "ip-b", Address: "10.0.0.2", AddressType: "INTERNAL", Status: "RESERVED", Region: "europe-west1", Subnetwork: "subnet-a"},
	}); err != nil {
		t.Fatalf("SaveIPAddresses() error = %v", err)
	}
	got, err := cache.GetIPAddresses("p1", "ip-a", "10.0.0.1")
	if err != nil {
		t.Fatalf("GetIPAddresses() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetIPAddresses() len = %d, want 2 (one per subnetwork)", len(got))
	}
	got, err = cache.GetIPAddresses("p1", "missing", "0.0.0.0")
	if err != nil {
		t.Fatalf("GetIPAddresses() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("GetIPAddresses() len = %d, want 0", len(got))
	}
}

func TestSaveAndListNetworks(t *testing.T) {
	cache := newTestCache(t)
	networks := []Network{
		{Name: "vpc-b", RoutingMode: "GLOBAL", AutoCreateSubnetworks: false, MTU: 1500, Description: "second"},
		{Name: "vpc-a", RoutingMode: "REGIONAL", AutoCreateSubnetworks: true, MTU: 1460, Description: "first"},
	}
	subnetworks := []Subnetwork{
		{Name: "sn-b", Region: "us-central1", Network: "vpc-a", IPRange: "10.1.0.0/24", PrivateIPGoogleAccess: true, Purpose: "INTERNAL_HTTPS_LOAD_BALANCER", Status: "READY"},
		{Name: "sn-a", Region: "europe-west1", Network: "vpc-a", IPRange: "10.0.0.0/24", SecondaryRanges: []string{"pods=10.64.0.0/14", "services=10.65.0.0/20"}, Status: "READY"},
	}
	if err := cache.SaveNetworks("p1", networks, subnetworks); err != nil {
		t.Fatalf("SaveNetworks() error = %v", err)
	}
	if err := cache.SaveNetworks("p2", networks[:1], nil); err != nil {
		t.Fatalf("SaveNetworks() other project error = %v", err)
	}

	gotNetworks, err := cache.ListNetworks("p1")
	if err != nil {
		t.Fatalf("ListNetworks() error = %v", err)
	}
	if len(gotNetworks) != 2 {
		t.Fatalf("ListNetworks() len = %d, want 2", len(gotNetworks))
	}
	if gotNetworks[0].Name != "vpc-a" || gotNetworks[1].Name != "vpc-b" {
		t.Fatalf("ListNetworks() not ordered by name: %+v", gotNetworks)
	}
	if !gotNetworks[0].AutoCreateSubnetworks || gotNetworks[0].RoutingMode != "REGIONAL" || gotNetworks[0].MTU != 1460 {
		t.Fatalf("ListNetworks()[0] = %+v", gotNetworks[0])
	}
	if gotNetworks[0].ProjectID != "p1" || gotNetworks[0].UpdatedAt == "" {
		t.Fatalf("ListNetworks()[0] missing project id or updated at: %+v", gotNetworks[0])
	}

	gotSubnets, err := cache.ListSubnetworks("p1")
	if err != nil {
		t.Fatalf("ListSubnetworks() error = %v", err)
	}
	if len(gotSubnets) != 2 {
		t.Fatalf("ListSubnetworks() len = %d, want 2", len(gotSubnets))
	}
	if gotSubnets[0].Region != "europe-west1" || gotSubnets[0].Name != "sn-a" {
		t.Fatalf("ListSubnetworks() not ordered by region, name: %+v", gotSubnets)
	}
	if len(gotSubnets[0].SecondaryRanges) != 2 || gotSubnets[0].SecondaryRanges[0] != "pods=10.64.0.0/14" {
		t.Fatalf("ListSubnetworks()[0].SecondaryRanges = %v", gotSubnets[0].SecondaryRanges)
	}
	if !gotSubnets[1].PrivateIPGoogleAccess || gotSubnets[1].Purpose != "INTERNAL_HTTPS_LOAD_BALANCER" {
		t.Fatalf("ListSubnetworks()[1] = %+v", gotSubnets[1])
	}

	if err := cache.SaveNetworks("p1", networks[:1], nil); err != nil {
		t.Fatalf("SaveNetworks() replace error = %v", err)
	}
	gotSubnets, err = cache.ListSubnetworks("p1")
	if err != nil {
		t.Fatalf("ListSubnetworks() error = %v", err)
	}
	if len(gotSubnets) != 0 {
		t.Fatalf("ListSubnetworks() len = %d, want 0 after replace", len(gotSubnets))
	}
	gotNetworks, err = cache.ListNetworks("p2")
	if err != nil {
		t.Fatalf("ListNetworks() error = %v", err)
	}
	if len(gotNetworks) != 1 {
		t.Fatalf("ListNetworks(p2) len = %d, want 1 (other project untouched)", len(gotNetworks))
	}
}

func TestGetNetworkAndSubnetwork(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.SaveNetworks("p1",
		[]Network{{Name: "vpc-a", RoutingMode: "REGIONAL"}},
		[]Subnetwork{
			{Name: "sn-a", Region: "europe-west1", Network: "vpc-a", IPRange: "10.0.0.0/24"},
			{Name: "sn-a", Region: "us-central1", Network: "vpc-a", IPRange: "10.1.0.0/24"},
		}); err != nil {
		t.Fatalf("SaveNetworks() error = %v", err)
	}
	n, err := cache.GetNetwork("p1", "vpc-a")
	if err != nil {
		t.Fatalf("GetNetwork() error = %v", err)
	}
	if n == nil || n.RoutingMode != "REGIONAL" {
		t.Fatalf("GetNetwork() = %+v", n)
	}
	s, err := cache.GetSubnetwork("p1", "us-central1", "sn-a")
	if err != nil {
		t.Fatalf("GetSubnetwork() error = %v", err)
	}
	if s == nil || s.IPRange != "10.1.0.0/24" {
		t.Fatalf("GetSubnetwork() = %+v, want us-central1 range", s)
	}
	n, err = cache.GetNetwork("p1", "missing")
	if err != nil {
		t.Fatalf("GetNetwork() error = %v", err)
	}
	if n != nil {
		t.Fatalf("GetNetwork() = %+v, want nil", n)
	}
	s, err = cache.GetSubnetwork("p1", "europe-west1", "missing")
	if err != nil {
		t.Fatalf("GetSubnetwork() error = %v", err)
	}
	if s != nil {
		t.Fatalf("GetSubnetwork() = %+v, want nil", s)
	}
}

func TestSaveAndListRoutes(t *testing.T) {
	cache := newTestCache(t)
	routes := []Route{
		{Name: "route-b", Network: "vpc-a", DestRange: "10.0.0.0/24", Priority: 900, NextHopKind: "gateway", NextHop: "default-internet-gateway", Description: "second"},
		{Name: "route-a", Network: "vpc-a", DestRange: "0.0.0.0/0", Priority: 1000, NextHopKind: "peering", NextHop: "peer-a"},
	}
	if err := cache.SaveRoutes("p1", routes); err != nil {
		t.Fatalf("SaveRoutes() error = %v", err)
	}
	if err := cache.SaveRoutes("p2", routes[:1]); err != nil {
		t.Fatalf("SaveRoutes() other project error = %v", err)
	}

	got, err := cache.ListRoutes("p1")
	if err != nil {
		t.Fatalf("ListRoutes() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListRoutes() len = %d, want 2", len(got))
	}
	if got[0].Name != "route-a" || got[1].Name != "route-b" {
		t.Fatalf("ListRoutes() not ordered by name: %+v", got)
	}
	if got[0].ProjectID != "p1" || got[0].Priority != 1000 || got[0].NextHopKind != "peering" || got[0].NextHop != "peer-a" {
		t.Fatalf("ListRoutes()[0] = %+v", got[0])
	}
	if got[1].Description != "second" {
		t.Fatalf("ListRoutes()[1] = %+v", got[1])
	}
	if got[0].UpdatedAt == "" {
		t.Fatalf("ListRoutes()[0] missing updated at: %+v", got[0])
	}

	all, err := cache.ListAllRoutes()
	if err != nil {
		t.Fatalf("ListAllRoutes() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAllRoutes() len = %d, want 3", len(all))
	}
	if all[0].ProjectID != "p1" || all[2].ProjectID != "p2" {
		t.Fatalf("ListAllRoutes() not ordered by project: %+v", all)
	}

	r, err := cache.GetRoute("p1", "route-a")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if r == nil || r.DestRange != "0.0.0.0/0" {
		t.Fatalf("GetRoute() = %+v", r)
	}
	r, err = cache.GetRoute("p1", "missing")
	if err != nil {
		t.Fatalf("GetRoute() error = %v", err)
	}
	if r != nil {
		t.Fatalf("GetRoute() = %+v, want nil", r)
	}

	if err := cache.SaveRoutes("p1", routes[:1]); err != nil {
		t.Fatalf("SaveRoutes() replace error = %v", err)
	}
	got, err = cache.ListRoutes("p1")
	if err != nil {
		t.Fatalf("ListRoutes() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListRoutes() len = %d, want 1 after replace", len(got))
	}
	got, err = cache.ListRoutes("p2")
	if err != nil {
		t.Fatalf("ListRoutes(p2) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListRoutes(p2) len = %d, want 1 (other project untouched)", len(got))
	}
}

func TestSaveAndListFirewallRules(t *testing.T) {
	cache := newTestCache(t)
	rules := []FirewallRule{
		{Name: "fw-b", Network: "vpc-a", Direction: "EGRESS", Priority: 1000, Action: "allow tcp:443", Source: "10.0.0.0/8", Targets: "web", Disabled: true, Description: "second"},
		{Name: "fw-a", Network: "vpc-a", Direction: "INGRESS", Priority: 900, Action: "deny esp", Source: "0.0.0.0/0"},
	}
	if err := cache.SaveFirewallRules("p1", rules); err != nil {
		t.Fatalf("SaveFirewallRules() error = %v", err)
	}
	if err := cache.SaveFirewallRules("p2", rules[:1]); err != nil {
		t.Fatalf("SaveFirewallRules() other project error = %v", err)
	}

	got, err := cache.ListFirewallRules("p1")
	if err != nil {
		t.Fatalf("ListFirewallRules() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListFirewallRules() len = %d, want 2", len(got))
	}
	if got[0].Name != "fw-a" || got[1].Name != "fw-b" {
		t.Fatalf("ListFirewallRules() not ordered by name: %+v", got)
	}
	if got[0].ProjectID != "p1" || got[0].Priority != 900 || got[0].Action != "deny esp" || got[0].Source != "0.0.0.0/0" {
		t.Fatalf("ListFirewallRules()[0] = %+v", got[0])
	}
	if !got[1].Disabled || got[1].Targets != "web" || got[1].Description != "second" {
		t.Fatalf("ListFirewallRules()[1] = %+v", got[1])
	}
	if got[0].UpdatedAt == "" {
		t.Fatalf("ListFirewallRules()[0] missing updated at: %+v", got[0])
	}

	all, err := cache.ListAllFirewallRules()
	if err != nil {
		t.Fatalf("ListAllFirewallRules() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAllFirewallRules() len = %d, want 3", len(all))
	}
	if all[0].ProjectID != "p1" || all[2].ProjectID != "p2" {
		t.Fatalf("ListAllFirewallRules() not ordered by project: %+v", all)
	}

	r, err := cache.GetFirewallRule("p1", "fw-a")
	if err != nil {
		t.Fatalf("GetFirewallRule() error = %v", err)
	}
	if r == nil || r.Direction != "INGRESS" {
		t.Fatalf("GetFirewallRule() = %+v", r)
	}
	r, err = cache.GetFirewallRule("p1", "missing")
	if err != nil {
		t.Fatalf("GetFirewallRule() error = %v", err)
	}
	if r != nil {
		t.Fatalf("GetFirewallRule() = %+v, want nil", r)
	}

	if err := cache.SaveFirewallRules("p1", rules[:1]); err != nil {
		t.Fatalf("SaveFirewallRules() replace error = %v", err)
	}
	got, err = cache.ListFirewallRules("p1")
	if err != nil {
		t.Fatalf("ListFirewallRules() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListFirewallRules() len = %d, want 1 after replace", len(got))
	}
	got, err = cache.ListFirewallRules("p2")
	if err != nil {
		t.Fatalf("ListFirewallRules(p2) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListFirewallRules(p2) len = %d, want 1 (other project untouched)", len(got))
	}
}

func TestListAllIPAddresses(t *testing.T) {
	cache := newTestCache(t)
	if err := cache.SaveIPAddresses("p2", []IPAddress{
		{ProjectID: "p2", Name: "ip-x", Address: "10.0.0.9", AddressType: "INTERNAL", Status: "RESERVED", Region: "europe-west1"},
	}); err != nil {
		t.Fatalf("SaveIPAddresses() error = %v", err)
	}
	if err := cache.SaveIPAddresses("p1", []IPAddress{
		{ProjectID: "p1", Name: "ip-a", Address: "10.0.0.1", AddressType: "INTERNAL", Status: "RESERVED", Region: "europe-west1"},
	}); err != nil {
		t.Fatalf("SaveIPAddresses() error = %v", err)
	}
	got, err := cache.ListAllIPAddresses()
	if err != nil {
		t.Fatalf("ListAllIPAddresses() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListAllIPAddresses() len = %d, want 2", len(got))
	}
	if got[0].ProjectID != "p1" {
		t.Fatalf("ListAllIPAddresses() not ordered by project, first = %+v", got[0])
	}
	if got[1].ProjectID != "p2" {
		t.Fatalf("ListAllIPAddresses() second = %+v, want p2", got[1])
	}
}

func TestSaveAndListCloudRouters(t *testing.T) {
	cache := newTestCache(t)
	routers := []CloudRouter{
		{
			Name: "router-b", Region: "europe-west1", Network: "vpc-a", Description: "second", BGPASN: 64514,
			Peers: []CloudRouterPeer{{Name: "peer-b", Interface: "if-0", IPAddress: "169.254.0.2", PeerIP: "169.254.0.1", PeerASN: 65001, Priority: 100}},
			NATs:  []string{"nat-b"},
		},
		{Name: "router-a", Region: "us-central1", Network: "vpc-a", Description: "first", BGPASN: 64512},
	}
	if err := cache.SaveCloudRouters("p1", routers); err != nil {
		t.Fatalf("SaveCloudRouters() error = %v", err)
	}
	if err := cache.SaveCloudRouters("p2", routers[:1]); err != nil {
		t.Fatalf("SaveCloudRouters() other project error = %v", err)
	}

	got, err := cache.ListCloudRouters("p1")
	if err != nil {
		t.Fatalf("ListCloudRouters() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListCloudRouters() len = %d, want 2", len(got))
	}
	if got[0].Name != "router-a" || got[1].Name != "router-b" {
		t.Fatalf("ListCloudRouters() not ordered by name: %+v", got)
	}
	if got[0].ProjectID != "p1" || got[0].BGPASN != 64512 {
		t.Fatalf("ListCloudRouters()[0] = %+v", got[0])
	}
	if len(got[1].Peers) != 1 || got[1].Peers[0].Name != "peer-b" || got[1].Peers[0].PeerASN != 65001 || got[1].Peers[0].PeerIP != "169.254.0.1" {
		t.Fatalf("ListCloudRouters()[1] peers = %+v", got[1].Peers)
	}
	if len(got[1].NATs) != 1 || got[1].NATs[0] != "nat-b" {
		t.Fatalf("ListCloudRouters()[1] nats = %+v", got[1].NATs)
	}
	if got[1].Description != "second" {
		t.Fatalf("ListCloudRouters()[1] = %+v", got[1])
	}
	if got[0].UpdatedAt == "" {
		t.Fatalf("ListCloudRouters()[0] missing updated at: %+v", got[0])
	}

	all, err := cache.ListAllCloudRouters()
	if err != nil {
		t.Fatalf("ListAllCloudRouters() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAllCloudRouters() len = %d, want 3", len(all))
	}
	if all[0].ProjectID != "p1" || all[2].ProjectID != "p2" {
		t.Fatalf("ListAllCloudRouters() not ordered by project: %+v", all)
	}

	r, err := cache.GetCloudRouter("p1", "europe-west1", "router-b")
	if err != nil {
		t.Fatalf("GetCloudRouter() error = %v", err)
	}
	if r == nil || r.BGPASN != 64514 || len(r.Peers) != 1 {
		t.Fatalf("GetCloudRouter() = %+v", r)
	}
	r, err = cache.GetCloudRouter("p1", "europe-west1", "missing")
	if err != nil {
		t.Fatalf("GetCloudRouter() error = %v", err)
	}
	if r != nil {
		t.Fatalf("GetCloudRouter() = %+v, want nil", r)
	}

	if err := cache.SaveCloudRouters("p1", routers[:1]); err != nil {
		t.Fatalf("SaveCloudRouters() replace error = %v", err)
	}
	got, err = cache.ListCloudRouters("p1")
	if err != nil {
		t.Fatalf("ListCloudRouters() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListCloudRouters() len = %d, want 1 after replace", len(got))
	}
	got, err = cache.ListCloudRouters("p2")
	if err != nil {
		t.Fatalf("ListCloudRouters(p2) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListCloudRouters(p2) len = %d, want 1 (other project untouched)", len(got))
	}
}

func TestSaveAndListVPNGateways(t *testing.T) {
	cache := newTestCache(t)
	gateways := []VPNGateway{
		{
			Name: "gw-b", Type: "ha", Region: "europe-west1", Network: "vpc-a", Description: "second",
			IPs: []string{"34.1.1.1", "34.1.1.2"},
			Tunnels: []VPNTunnel{
				{Name: "tunnel-1", Interface: 0, PeerIP: "203.0.113.1", IKE: 2, Status: "ESTABLISHED", Router: "router-a"},
				{Name: "tunnel-2", Interface: 1, PeerIP: "203.0.113.2", IKE: 2, Status: "DOWN", Router: "router-a"},
			},
		},
		{Name: "gw-a", Type: "classic", Region: "us-central1", Network: "vpc-a", Description: "first"},
	}
	if err := cache.SaveVPNGateways("p1", gateways); err != nil {
		t.Fatalf("SaveVPNGateways() error = %v", err)
	}
	if err := cache.SaveVPNGateways("p2", gateways[:1]); err != nil {
		t.Fatalf("SaveVPNGateways() other project error = %v", err)
	}

	got, err := cache.ListVPNGateways("p1")
	if err != nil {
		t.Fatalf("ListVPNGateways() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListVPNGateways() len = %d, want 2", len(got))
	}
	if got[0].Name != "gw-a" || got[1].Name != "gw-b" {
		t.Fatalf("ListVPNGateways() not ordered by name: %+v", got)
	}
	if got[0].ProjectID != "p1" || got[0].Type != "classic" || len(got[0].IPs) != 0 {
		t.Fatalf("ListVPNGateways()[0] = %+v", got[0])
	}
	if len(got[1].IPs) != 2 || got[1].IPs[0] != "34.1.1.1" {
		t.Fatalf("ListVPNGateways()[1] ips = %+v", got[1].IPs)
	}
	if len(got[1].Tunnels) != 2 || got[1].Tunnels[0].Name != "tunnel-1" || got[1].Tunnels[0].PeerIP != "203.0.113.1" || got[1].Tunnels[1].Status != "DOWN" {
		t.Fatalf("ListVPNGateways()[1] tunnels = %+v", got[1].Tunnels)
	}
	if got[1].Tunnels[0].IKE != 2 || got[1].Tunnels[0].Router != "router-a" || got[1].Tunnels[0].Interface != 0 {
		t.Fatalf("ListVPNGateways()[1] tunnel details = %+v", got[1].Tunnels[0])
	}
	if got[1].Description != "second" {
		t.Fatalf("ListVPNGateways()[1] = %+v", got[1])
	}
	if got[0].UpdatedAt == "" {
		t.Fatalf("ListVPNGateways()[0] missing updated at: %+v", got[0])
	}

	all, err := cache.ListAllVPNGateways()
	if err != nil {
		t.Fatalf("ListAllVPNGateways() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAllVPNGateways() len = %d, want 3", len(all))
	}
	if all[0].ProjectID != "p1" || all[2].ProjectID != "p2" {
		t.Fatalf("ListAllVPNGateways() not ordered by project: %+v", all)
	}

	g, err := cache.GetVPNGateway("p1", "europe-west1", "gw-b")
	if err != nil {
		t.Fatalf("GetVPNGateway() error = %v", err)
	}
	if g == nil || g.Type != "ha" || len(g.Tunnels) != 2 {
		t.Fatalf("GetVPNGateway() = %+v", g)
	}
	g, err = cache.GetVPNGateway("p1", "europe-west1", "missing")
	if err != nil {
		t.Fatalf("GetVPNGateway() error = %v", err)
	}
	if g != nil {
		t.Fatalf("GetVPNGateway() = %+v, want nil", g)
	}

	if err := cache.SaveVPNGateways("p1", gateways[:1]); err != nil {
		t.Fatalf("SaveVPNGateways() replace error = %v", err)
	}
	got, err = cache.ListVPNGateways("p1")
	if err != nil {
		t.Fatalf("ListVPNGateways() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListVPNGateways() len = %d, want 1 after replace", len(got))
	}
	got, err = cache.ListVPNGateways("p2")
	if err != nil {
		t.Fatalf("ListVPNGateways(p2) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListVPNGateways(p2) len = %d, want 1 (other project untouched)", len(got))
	}
}
