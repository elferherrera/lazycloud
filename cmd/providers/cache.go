package providers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Cache struct {
	db *sql.DB
}

func NewCache() (*Cache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return NewCacheAt(filepath.Join(home, ".config", "lazycloud", "cache.db"))
}

func NewCacheAt(path string) (*Cache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS projects (
		project_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		project_number TEXT NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureIPAddressesPrimaryKey(db); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS ip_addresses (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		address TEXT NOT NULL,
		address_type TEXT NOT NULL,
		status TEXT NOT NULL,
		region TEXT NOT NULL,
		subnetwork TEXT NOT NULL DEFAULT '',
		network TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name, address, subnetwork, network)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	for _, col := range []struct{ name, definition string }{
		{"subnetwork", "TEXT NOT NULL DEFAULT ''"},
		{"network", "TEXT NOT NULL DEFAULT ''"},
		{"updated_at", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := ensureColumn(db, "ip_addresses", col.name, col.definition); err != nil {
			db.Close()
			return nil, err
		}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS deleted_projects (
		project_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		project_number TEXT NOT NULL,
		deleted_at TEXT NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS networks (
		project_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		name TEXT NOT NULL,
		region TEXT NOT NULL DEFAULT '',
		network TEXT NOT NULL DEFAULT '',
		ip_range TEXT NOT NULL DEFAULT '',
		secondary_ranges TEXT NOT NULL DEFAULT '',
		private_ip_google_access INTEGER NOT NULL DEFAULT 0,
		purpose TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		routing_mode TEXT NOT NULL DEFAULT '',
		auto_create_subnets INTEGER NOT NULL DEFAULT 0,
		mtu INTEGER NOT NULL DEFAULT 0,
		description TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, kind, name, region)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS service_accounts (
		project_id TEXT NOT NULL,
		email TEXT NOT NULL,
		display_name TEXT NOT NULL DEFAULT '',
		unique_id TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		disabled INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, email)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS vms (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		instance_id TEXT NOT NULL DEFAULT '',
		zone TEXT NOT NULL,
		machine_type TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		interfaces TEXT NOT NULL DEFAULT '[]',
		boot_disk TEXT NOT NULL DEFAULT 'null',
		data_disks TEXT NOT NULL DEFAULT '[]',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name, zone)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureColumn(db, "vms", "instance_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS disks (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		zone TEXT NOT NULL,
		size_gb INTEGER NOT NULL DEFAULT 0,
		type TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		source_image TEXT NOT NULL DEFAULT '',
		users TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name, zone)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Cache{db: db}, nil
}

func ensureIPAddressesPrimaryKey(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(ip_addresses)")
	if err != nil {
		return err
	}
	defer rows.Close()
	positions := map[int]string{}
	found := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		found = true
		if pk > 0 {
			positions[pk] = name
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !found {
		return nil
	}
	current := make([]string, 0, len(positions))
	for i := 1; ; i++ {
		name, ok := positions[i]
		if !ok {
			break
		}
		current = append(current, name)
	}
	desired := []string{"project_id", "name", "address", "subnetwork", "network"}
	if slices.Equal(current, desired) {
		return nil
	}
	_, err = db.Exec("DROP TABLE ip_addresses")
	return err
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	_, err = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}

func (c *Cache) SaveProjects(projects []Project) error {
	deleted, err := c.ListDeletedProjects()
	if err != nil {
		return err
	}
	deletedIDs := make(map[string]bool, len(deleted))
	for _, d := range deleted {
		deletedIDs[d.ProjectID] = true
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM projects"); err != nil {
		return err
	}
	for _, p := range projects {
		if deletedIDs[p.ProjectID] {
			continue
		}
		if _, err := tx.Exec("INSERT INTO projects (project_id, name, project_number) VALUES (?, ?, ?)",
			p.ProjectID, p.Name, p.ProjectNumber); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *Cache) ListProjects() ([]Project, error) {
	rows, err := c.db.Query("SELECT project_id, name, project_number FROM projects ORDER BY project_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ProjectID, &p.Name, &p.ProjectNumber); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (c *Cache) SaveIPAddresses(projectID string, addresses []IPAddress) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	for _, a := range addresses {
		if _, err := tx.Exec(`INSERT INTO ip_addresses
			(project_id, name, address, address_type, status, region, subnetwork, network, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (project_id, name, address, subnetwork, network) DO UPDATE SET
			address_type = excluded.address_type,
			status = excluded.status,
			region = excluded.region,
			updated_at = excluded.updated_at`,
			projectID, a.Name, a.Address, a.AddressType, a.Status, a.Region, a.Subnetwork, a.Network, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *Cache) ListIPAddresses(projectID string) ([]IPAddress, error) {
	rows, err := c.db.Query("SELECT project_id, name, address, address_type, status, region, subnetwork, network, updated_at FROM ip_addresses WHERE project_id = ? ORDER BY name", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var addresses []IPAddress
	for rows.Next() {
		var a IPAddress
		if err := rows.Scan(&a.ProjectID, &a.Name, &a.Address, &a.AddressType, &a.Status, &a.Region, &a.Subnetwork, &a.Network, &a.UpdatedAt); err != nil {
			return nil, err
		}
		addresses = append(addresses, a)
	}
	return addresses, rows.Err()
}

func (c *Cache) ListAllIPAddresses() ([]IPAddress, error) {
	rows, err := c.db.Query("SELECT project_id, name, address, address_type, status, region, subnetwork, network, updated_at FROM ip_addresses ORDER BY project_id, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var addresses []IPAddress
	for rows.Next() {
		var a IPAddress
		if err := rows.Scan(&a.ProjectID, &a.Name, &a.Address, &a.AddressType, &a.Status, &a.Region, &a.Subnetwork, &a.Network, &a.UpdatedAt); err != nil {
			return nil, err
		}
		addresses = append(addresses, a)
	}
	return addresses, rows.Err()
}

func (c *Cache) GetProject(projectID string) (*Project, error) {
	row := c.db.QueryRow("SELECT project_id, name, project_number FROM projects WHERE project_id = ?", projectID)
	var p Project
	if err := row.Scan(&p.ProjectID, &p.Name, &p.ProjectNumber); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (c *Cache) GetIPAddresses(projectID, name, address string) ([]IPAddress, error) {
	rows, err := c.db.Query("SELECT project_id, name, address, address_type, status, region, subnetwork, network, updated_at FROM ip_addresses WHERE project_id = ? AND name = ? AND address = ?", projectID, name, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var addresses []IPAddress
	for rows.Next() {
		var a IPAddress
		if err := rows.Scan(&a.ProjectID, &a.Name, &a.Address, &a.AddressType, &a.Status, &a.Region, &a.Subnetwork, &a.Network, &a.UpdatedAt); err != nil {
			return nil, err
		}
		addresses = append(addresses, a)
	}
	return addresses, rows.Err()
}

func (c *Cache) SaveNetworks(projectID string, networks []Network, subnetworks []Subnetwork) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM networks WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, n := range networks {
		if _, err := tx.Exec(`INSERT INTO networks
			(project_id, kind, name, routing_mode, auto_create_subnets, mtu, description, updated_at)
			VALUES (?, 'vpc', ?, ?, ?, ?, ?, ?)`,
			projectID, n.Name, n.RoutingMode, n.AutoCreateSubnetworks, n.MTU, n.Description, updatedAt); err != nil {
			return err
		}
	}
	for _, s := range subnetworks {
		if _, err := tx.Exec(`INSERT INTO networks
			(project_id, kind, name, region, network, ip_range, secondary_ranges, private_ip_google_access, purpose, status, description, updated_at)
			VALUES (?, 'subnet', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, s.Name, s.Region, s.Network, s.IPRange, strings.Join(s.SecondaryRanges, ","),
			s.PrivateIPGoogleAccess, s.Purpose, s.Status, s.Description, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *Cache) ListNetworks(projectID string) ([]Network, error) {
	rows, err := c.db.Query(`SELECT project_id, name, routing_mode, auto_create_subnets, mtu, description, updated_at
		FROM networks WHERE project_id = ? AND kind = 'vpc' ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var networks []Network
	for rows.Next() {
		var n Network
		var autoCreate int
		if err := rows.Scan(&n.ProjectID, &n.Name, &n.RoutingMode, &autoCreate, &n.MTU, &n.Description, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.AutoCreateSubnetworks = autoCreate != 0
		networks = append(networks, n)
	}
	return networks, rows.Err()
}

func (c *Cache) ListSubnetworks(projectID string) ([]Subnetwork, error) {
	rows, err := c.db.Query(`SELECT project_id, name, region, network, ip_range, secondary_ranges, private_ip_google_access, purpose, status, updated_at
		FROM networks WHERE project_id = ? AND kind = 'subnet' ORDER BY region, name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subnetworks []Subnetwork
	for rows.Next() {
		var s Subnetwork
		var privateAccess int
		var secondary string
		if err := rows.Scan(&s.ProjectID, &s.Name, &s.Region, &s.Network, &s.IPRange, &secondary,
			&privateAccess, &s.Purpose, &s.Status, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.PrivateIPGoogleAccess = privateAccess != 0
		if secondary != "" {
			s.SecondaryRanges = strings.Split(secondary, ",")
		}
		subnetworks = append(subnetworks, s)
	}
	return subnetworks, rows.Err()
}

func (c *Cache) ListAllNetworks() ([]Network, error) {
	rows, err := c.db.Query(`SELECT project_id, name, routing_mode, auto_create_subnets, mtu, description, updated_at
		FROM networks WHERE kind = 'vpc' ORDER BY project_id, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var networks []Network
	for rows.Next() {
		var n Network
		var autoCreate int
		if err := rows.Scan(&n.ProjectID, &n.Name, &n.RoutingMode, &autoCreate, &n.MTU, &n.Description, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.AutoCreateSubnetworks = autoCreate != 0
		networks = append(networks, n)
	}
	return networks, rows.Err()
}

func (c *Cache) ListAllSubnetworks() ([]Subnetwork, error) {
	rows, err := c.db.Query(`SELECT project_id, name, region, network, ip_range, secondary_ranges, private_ip_google_access, purpose, status, updated_at
		FROM networks WHERE kind = 'subnet' ORDER BY project_id, region, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subnetworks []Subnetwork
	for rows.Next() {
		var s Subnetwork
		var privateAccess int
		var secondary string
		if err := rows.Scan(&s.ProjectID, &s.Name, &s.Region, &s.Network, &s.IPRange, &secondary,
			&privateAccess, &s.Purpose, &s.Status, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.PrivateIPGoogleAccess = privateAccess != 0
		if secondary != "" {
			s.SecondaryRanges = strings.Split(secondary, ",")
		}
		subnetworks = append(subnetworks, s)
	}
	return subnetworks, rows.Err()
}

func (c *Cache) GetNetwork(projectID, name string) (*Network, error) {
	row := c.db.QueryRow(`SELECT project_id, name, routing_mode, auto_create_subnets, mtu, description, updated_at
		FROM networks WHERE project_id = ? AND kind = 'vpc' AND name = ?`, projectID, name)
	var n Network
	var autoCreate int
	if err := row.Scan(&n.ProjectID, &n.Name, &n.RoutingMode, &autoCreate, &n.MTU, &n.Description, &n.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	n.AutoCreateSubnetworks = autoCreate != 0
	return &n, nil
}

func (c *Cache) GetSubnetwork(projectID, region, name string) (*Subnetwork, error) {
	row := c.db.QueryRow(`SELECT project_id, name, region, network, ip_range, secondary_ranges, private_ip_google_access, purpose, status, updated_at
		FROM networks WHERE project_id = ? AND kind = 'subnet' AND name = ? AND region = ?`, projectID, name, region)
	var s Subnetwork
	var privateAccess int
	var secondary string
	if err := row.Scan(&s.ProjectID, &s.Name, &s.Region, &s.Network, &s.IPRange, &secondary,
		&privateAccess, &s.Purpose, &s.Status, &s.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	s.PrivateIPGoogleAccess = privateAccess != 0
	if secondary != "" {
		s.SecondaryRanges = strings.Split(secondary, ",")
	}
	return &s, nil
}

func (c *Cache) DeleteProject(projectID string) (*Project, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var p Project
	if err := tx.QueryRow("SELECT project_id, name, project_number FROM projects WHERE project_id = ?", projectID).
		Scan(&p.ProjectID, &p.Name, &p.ProjectNumber); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	for _, table := range []string{"projects", "ip_addresses", "networks", "service_accounts", "vms", "disks"} {
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE project_id = ?", projectID); err != nil {
			return nil, err
		}
	}
	deletedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(`INSERT INTO deleted_projects (project_id, name, project_number, deleted_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (project_id) DO UPDATE SET
		name = excluded.name,
		project_number = excluded.project_number,
		deleted_at = excluded.deleted_at`,
		p.ProjectID, p.Name, p.ProjectNumber, deletedAt); err != nil {
		return nil, err
	}
	return &p, tx.Commit()
}

func (c *Cache) RestoreProject() (*Project, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var p Project
	if err := tx.QueryRow("SELECT project_id, name, project_number FROM deleted_projects ORDER BY deleted_at DESC LIMIT 1").
		Scan(&p.ProjectID, &p.Name, &p.ProjectNumber); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if _, err := tx.Exec("DELETE FROM deleted_projects WHERE project_id = ?", p.ProjectID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO projects (project_id, name, project_number) VALUES (?, ?, ?)
		ON CONFLICT (project_id) DO UPDATE SET
		name = excluded.name,
		project_number = excluded.project_number`,
		p.ProjectID, p.Name, p.ProjectNumber); err != nil {
		return nil, err
	}
	return &p, tx.Commit()
}

func (c *Cache) ListDeletedProjects() ([]Project, error) {
	rows, err := c.db.Query("SELECT project_id, name, project_number FROM deleted_projects ORDER BY project_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ProjectID, &p.Name, &p.ProjectNumber); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (c *Cache) SaveServiceAccounts(projectID string, accounts []ServiceAccount) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM service_accounts WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, sa := range accounts {
		if _, err := tx.Exec(`INSERT INTO service_accounts
			(project_id, email, display_name, unique_id, description, disabled, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			projectID, sa.Email, sa.DisplayName, sa.UniqueID, sa.Description, sa.Disabled, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *Cache) ListServiceAccounts(projectID string) ([]ServiceAccount, error) {
	rows, err := c.db.Query(`SELECT project_id, email, display_name, unique_id, description, disabled, updated_at
		FROM service_accounts WHERE project_id = ? ORDER BY email`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []ServiceAccount
	for rows.Next() {
		var sa ServiceAccount
		var disabled int
		if err := rows.Scan(&sa.ProjectID, &sa.Email, &sa.DisplayName, &sa.UniqueID, &sa.Description, &disabled, &sa.UpdatedAt); err != nil {
			return nil, err
		}
		sa.Disabled = disabled != 0
		accounts = append(accounts, sa)
	}
	return accounts, rows.Err()
}

func (c *Cache) ListAllServiceAccounts() ([]ServiceAccount, error) {
	rows, err := c.db.Query(`SELECT project_id, email, display_name, unique_id, description, disabled, updated_at
		FROM service_accounts ORDER BY project_id, email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []ServiceAccount
	for rows.Next() {
		var sa ServiceAccount
		var disabled int
		if err := rows.Scan(&sa.ProjectID, &sa.Email, &sa.DisplayName, &sa.UniqueID, &sa.Description, &disabled, &sa.UpdatedAt); err != nil {
			return nil, err
		}
		sa.Disabled = disabled != 0
		accounts = append(accounts, sa)
	}
	return accounts, rows.Err()
}

func (c *Cache) GetServiceAccount(projectID, email string) (*ServiceAccount, error) {
	row := c.db.QueryRow(`SELECT project_id, email, display_name, unique_id, description, disabled, updated_at
		FROM service_accounts WHERE project_id = ? AND email = ?`, projectID, email)
	var sa ServiceAccount
	var disabled int
	if err := row.Scan(&sa.ProjectID, &sa.Email, &sa.DisplayName, &sa.UniqueID, &sa.Description, &disabled, &sa.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	sa.Disabled = disabled != 0
	return &sa, nil
}

func (c *Cache) SaveVMs(projectID string, vms []VM) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM vms WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, vm := range vms {
		interfaces, err := json.Marshal(vm.Interfaces)
		if err != nil {
			return err
		}
		bootDisk, err := json.Marshal(vm.BootDisk)
		if err != nil {
			return err
		}
		dataDisks, err := json.Marshal(vm.DataDisks)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO vms
			(project_id, name, instance_id, zone, machine_type, status, interfaces, boot_disk, data_disks, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, vm.Name, vm.InstanceID, vm.Zone, vm.MachineType, vm.Status,
			string(interfaces), string(bootDisk), string(dataDisks), updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanVMs(rows *sql.Rows) ([]VM, error) {
	var vms []VM
	for rows.Next() {
		var vm VM
		var interfaces, bootDisk, dataDisks string
		if err := rows.Scan(&vm.ProjectID, &vm.Name, &vm.InstanceID, &vm.Zone, &vm.MachineType, &vm.Status,
			&interfaces, &bootDisk, &dataDisks, &vm.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(interfaces), &vm.Interfaces); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(bootDisk), &vm.BootDisk); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(dataDisks), &vm.DataDisks); err != nil {
			return nil, err
		}
		vms = append(vms, vm)
	}
	return vms, rows.Err()
}

func (c *Cache) ListVMs(projectID string) ([]VM, error) {
	rows, err := c.db.Query(`SELECT project_id, name, instance_id, zone, machine_type, status, interfaces, boot_disk, data_disks, updated_at
		FROM vms WHERE project_id = ? ORDER BY zone, name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVMs(rows)
}

func (c *Cache) ListAllVMs() ([]VM, error) {
	rows, err := c.db.Query(`SELECT project_id, name, instance_id, zone, machine_type, status, interfaces, boot_disk, data_disks, updated_at
		FROM vms ORDER BY project_id, zone, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVMs(rows)
}

func (c *Cache) GetVM(projectID, zone, name string) (*VM, error) {
	rows, err := c.db.Query(`SELECT project_id, name, instance_id, zone, machine_type, status, interfaces, boot_disk, data_disks, updated_at
		FROM vms WHERE project_id = ? AND name = ? AND zone = ?`, projectID, name, zone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vms, err := scanVMs(rows)
	if err != nil {
		return nil, err
	}
	if len(vms) == 0 {
		return nil, nil
	}
	return &vms[0], nil
}

func (c *Cache) SaveDisks(projectID string, disks []Disk) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM disks WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, d := range disks {
		if _, err := tx.Exec(`INSERT INTO disks
			(project_id, name, zone, size_gb, type, status, source_image, users, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, d.Name, d.Zone, d.SizeGB, d.Type, d.Status, d.SourceImage,
			strings.Join(d.Users, ","), updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanDisks(rows *sql.Rows) ([]Disk, error) {
	var disks []Disk
	for rows.Next() {
		var d Disk
		var users string
		if err := rows.Scan(&d.ProjectID, &d.Name, &d.Zone, &d.SizeGB, &d.Type, &d.Status, &d.SourceImage, &users, &d.UpdatedAt); err != nil {
			return nil, err
		}
		if users != "" {
			d.Users = strings.Split(users, ",")
		}
		disks = append(disks, d)
	}
	return disks, rows.Err()
}

func (c *Cache) ListDisks(projectID string) ([]Disk, error) {
	rows, err := c.db.Query(`SELECT project_id, name, zone, size_gb, type, status, source_image, users, updated_at
		FROM disks WHERE project_id = ? ORDER BY zone, name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDisks(rows)
}

func (c *Cache) ListAllDisks() ([]Disk, error) {
	rows, err := c.db.Query(`SELECT project_id, name, zone, size_gb, type, status, source_image, users, updated_at
		FROM disks ORDER BY project_id, zone, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDisks(rows)
}

func (c *Cache) GetDisk(projectID, zone, name string) (*Disk, error) {
	rows, err := c.db.Query(`SELECT project_id, name, zone, size_gb, type, status, source_image, users, updated_at
		FROM disks WHERE project_id = ? AND name = ? AND zone = ?`, projectID, name, zone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	disks, err := scanDisks(rows)
	if err != nil {
		return nil, err
	}
	if len(disks) == 0 {
		return nil, nil
	}
	return &disks[0], nil
}

func (c *Cache) Close() error {
	return c.db.Close()
}
