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
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS routes (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		network TEXT NOT NULL DEFAULT '',
		dest_range TEXT NOT NULL DEFAULT '',
		priority INTEGER NOT NULL DEFAULT 0,
		next_hop_kind TEXT NOT NULL DEFAULT '',
		next_hop TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS firewall_rules (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		network TEXT NOT NULL DEFAULT '',
		direction TEXT NOT NULL DEFAULT '',
		priority INTEGER NOT NULL DEFAULT 0,
		action TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL DEFAULT '',
		targets TEXT NOT NULL DEFAULT '',
		disabled INTEGER NOT NULL DEFAULT 0,
		description TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name)
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
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS buckets (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		location TEXT NOT NULL DEFAULT '',
		storage_class TEXT NOT NULL DEFAULT '',
		versioning INTEGER NOT NULL DEFAULT 0,
		uniform_access INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS forwarding_rules (
		project_id TEXT NOT NULL,
		region TEXT NOT NULL,
		name TEXT NOT NULL,
		ip_address TEXT NOT NULL DEFAULT '',
		port_range TEXT NOT NULL DEFAULT '',
		protocol TEXT NOT NULL DEFAULT '',
		scheme TEXT NOT NULL DEFAULT '',
		target_kind TEXT NOT NULL DEFAULT '',
		target TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, region, name)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS target_proxies (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		kind TEXT NOT NULL DEFAULT '',
		url_map TEXT NOT NULL DEFAULT '',
		backend_service TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS url_maps (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		default_service TEXT NOT NULL DEFAULT '',
		path_rules TEXT NOT NULL DEFAULT '[]',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS backend_services (
		project_id TEXT NOT NULL,
		region TEXT NOT NULL,
		name TEXT NOT NULL,
		protocol TEXT NOT NULL DEFAULT '',
		port_name TEXT NOT NULL DEFAULT '',
		backends TEXT NOT NULL DEFAULT '[]',
		health_checks TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, region, name)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS clusters (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		location TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		version TEXT NOT NULL DEFAULT '',
		endpoint TEXT NOT NULL DEFAULT '',
		node_pools TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS workloads (
		project_id TEXT NOT NULL,
		cluster TEXT NOT NULL,
		kind TEXT NOT NULL,
		namespace TEXT NOT NULL,
		name TEXT NOT NULL,
		replicas TEXT NOT NULL DEFAULT '',
		images TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, cluster, kind, namespace, name)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS dns_zones (
		project_id TEXT NOT NULL,
		name TEXT NOT NULL,
		dns_name TEXT NOT NULL DEFAULT '',
		visibility TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, name)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS dns_records (
		project_id TEXT NOT NULL,
		zone TEXT NOT NULL,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		ttl INTEGER NOT NULL DEFAULT 0,
		data TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, zone, name, type)
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

func (c *Cache) SaveRoutes(projectID string, routes []Route) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM routes WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, r := range routes {
		if _, err := tx.Exec(`INSERT INTO routes
			(project_id, name, network, dest_range, priority, next_hop_kind, next_hop, description, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, r.Name, r.Network, r.DestRange, r.Priority, r.NextHopKind, r.NextHop, r.Description, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanRoutes(rows *sql.Rows) ([]Route, error) {
	defer rows.Close()
	var routes []Route
	for rows.Next() {
		var r Route
		if err := rows.Scan(&r.ProjectID, &r.Name, &r.Network, &r.DestRange, &r.Priority,
			&r.NextHopKind, &r.NextHop, &r.Description, &r.UpdatedAt); err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	return routes, rows.Err()
}

func (c *Cache) ListRoutes(projectID string) ([]Route, error) {
	rows, err := c.db.Query(`SELECT project_id, name, network, dest_range, priority, next_hop_kind, next_hop, description, updated_at
		FROM routes WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	return scanRoutes(rows)
}

func (c *Cache) ListAllRoutes() ([]Route, error) {
	rows, err := c.db.Query(`SELECT project_id, name, network, dest_range, priority, next_hop_kind, next_hop, description, updated_at
		FROM routes ORDER BY project_id, name`)
	if err != nil {
		return nil, err
	}
	return scanRoutes(rows)
}

func (c *Cache) GetRoute(projectID, name string) (*Route, error) {
	row := c.db.QueryRow(`SELECT project_id, name, network, dest_range, priority, next_hop_kind, next_hop, description, updated_at
		FROM routes WHERE project_id = ? AND name = ?`, projectID, name)
	var r Route
	if err := row.Scan(&r.ProjectID, &r.Name, &r.Network, &r.DestRange, &r.Priority,
		&r.NextHopKind, &r.NextHop, &r.Description, &r.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func (c *Cache) SaveFirewallRules(projectID string, rules []FirewallRule) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM firewall_rules WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, r := range rules {
		if _, err := tx.Exec(`INSERT INTO firewall_rules
			(project_id, name, network, direction, priority, action, source, targets, disabled, description, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, r.Name, r.Network, r.Direction, r.Priority, r.Action, r.Source, r.Targets,
			r.Disabled, r.Description, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanFirewallRules(rows *sql.Rows) ([]FirewallRule, error) {
	defer rows.Close()
	var rules []FirewallRule
	for rows.Next() {
		var r FirewallRule
		var disabled int
		if err := rows.Scan(&r.ProjectID, &r.Name, &r.Network, &r.Direction, &r.Priority,
			&r.Action, &r.Source, &r.Targets, &disabled, &r.Description, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Disabled = disabled != 0
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (c *Cache) ListFirewallRules(projectID string) ([]FirewallRule, error) {
	rows, err := c.db.Query(`SELECT project_id, name, network, direction, priority, action, source, targets, disabled, description, updated_at
		FROM firewall_rules WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	return scanFirewallRules(rows)
}

func (c *Cache) ListAllFirewallRules() ([]FirewallRule, error) {
	rows, err := c.db.Query(`SELECT project_id, name, network, direction, priority, action, source, targets, disabled, description, updated_at
		FROM firewall_rules ORDER BY project_id, name`)
	if err != nil {
		return nil, err
	}
	return scanFirewallRules(rows)
}

func (c *Cache) GetFirewallRule(projectID, name string) (*FirewallRule, error) {
	row := c.db.QueryRow(`SELECT project_id, name, network, direction, priority, action, source, targets, disabled, description, updated_at
		FROM firewall_rules WHERE project_id = ? AND name = ?`, projectID, name)
	var r FirewallRule
	var disabled int
	if err := row.Scan(&r.ProjectID, &r.Name, &r.Network, &r.Direction, &r.Priority,
		&r.Action, &r.Source, &r.Targets, &disabled, &r.Description, &r.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	r.Disabled = disabled != 0
	return &r, nil
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
	for _, table := range []string{"projects", "ip_addresses", "networks", "routes", "firewall_rules", "service_accounts", "vms", "disks", "buckets", "forwarding_rules", "target_proxies", "url_maps", "backend_services", "clusters", "workloads", "dns_zones", "dns_records"} {
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

func (c *Cache) SaveBuckets(projectID string, buckets []Bucket) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM buckets WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, b := range buckets {
		if _, err := tx.Exec(`INSERT INTO buckets
			(project_id, name, location, storage_class, versioning, uniform_access, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, b.Name, b.Location, b.StorageClass, b.Versioning, b.UniformAccess, b.CreatedAt, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanBuckets(rows *sql.Rows) ([]Bucket, error) {
	var buckets []Bucket
	for rows.Next() {
		var b Bucket
		var versioning, uniform int
		if err := rows.Scan(&b.ProjectID, &b.Name, &b.Location, &b.StorageClass, &versioning, &uniform, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.Versioning = versioning != 0
		b.UniformAccess = uniform != 0
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

func (c *Cache) ListBuckets(projectID string) ([]Bucket, error) {
	rows, err := c.db.Query(`SELECT project_id, name, location, storage_class, versioning, uniform_access, created_at, updated_at
		FROM buckets WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBuckets(rows)
}

func (c *Cache) ListAllBuckets() ([]Bucket, error) {
	rows, err := c.db.Query(`SELECT project_id, name, location, storage_class, versioning, uniform_access, created_at, updated_at
		FROM buckets ORDER BY project_id, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBuckets(rows)
}

func (c *Cache) GetBucket(projectID, name string) (*Bucket, error) {
	rows, err := c.db.Query(`SELECT project_id, name, location, storage_class, versioning, uniform_access, created_at, updated_at
		FROM buckets WHERE project_id = ? AND name = ?`, projectID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	buckets, err := scanBuckets(rows)
	if err != nil {
		return nil, err
	}
	if len(buckets) == 0 {
		return nil, nil
	}
	return &buckets[0], nil
}

func (c *Cache) SaveForwardingRules(projectID string, rules []ForwardingRule) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM forwarding_rules WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, r := range rules {
		if _, err := tx.Exec(`INSERT INTO forwarding_rules
			(project_id, region, name, ip_address, port_range, protocol, scheme, target_kind, target, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, r.Region, r.Name, r.IPAddress, r.PortRange, r.Protocol, r.Scheme, r.TargetKind, r.Target, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanForwardingRules(rows *sql.Rows) ([]ForwardingRule, error) {
	var rules []ForwardingRule
	for rows.Next() {
		var r ForwardingRule
		if err := rows.Scan(&r.ProjectID, &r.Region, &r.Name, &r.IPAddress, &r.PortRange, &r.Protocol, &r.Scheme, &r.TargetKind, &r.Target, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (c *Cache) ListForwardingRules(projectID string) ([]ForwardingRule, error) {
	rows, err := c.db.Query(`SELECT project_id, region, name, ip_address, port_range, protocol, scheme, target_kind, target, updated_at
		FROM forwarding_rules WHERE project_id = ? ORDER BY region, name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanForwardingRules(rows)
}

func (c *Cache) ListAllForwardingRules() ([]ForwardingRule, error) {
	rows, err := c.db.Query(`SELECT project_id, region, name, ip_address, port_range, protocol, scheme, target_kind, target, updated_at
		FROM forwarding_rules ORDER BY project_id, region, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanForwardingRules(rows)
}

func (c *Cache) GetForwardingRule(projectID, region, name string) (*ForwardingRule, error) {
	rows, err := c.db.Query(`SELECT project_id, region, name, ip_address, port_range, protocol, scheme, target_kind, target, updated_at
		FROM forwarding_rules WHERE project_id = ? AND region = ? AND name = ?`, projectID, region, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules, err := scanForwardingRules(rows)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	return &rules[0], nil
}

func (c *Cache) SaveTargetProxies(projectID string, proxies []TargetProxy) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM target_proxies WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, p := range proxies {
		if _, err := tx.Exec(`INSERT INTO target_proxies
			(project_id, name, kind, url_map, backend_service, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			projectID, p.Name, p.Kind, p.URLMap, p.BackendService, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanTargetProxies(rows *sql.Rows) ([]TargetProxy, error) {
	var proxies []TargetProxy
	for rows.Next() {
		var p TargetProxy
		if err := rows.Scan(&p.ProjectID, &p.Name, &p.Kind, &p.URLMap, &p.BackendService, &p.UpdatedAt); err != nil {
			return nil, err
		}
		proxies = append(proxies, p)
	}
	return proxies, rows.Err()
}

func (c *Cache) ListTargetProxies(projectID string) ([]TargetProxy, error) {
	rows, err := c.db.Query(`SELECT project_id, name, kind, url_map, backend_service, updated_at
		FROM target_proxies WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTargetProxies(rows)
}

func (c *Cache) ListAllTargetProxies() ([]TargetProxy, error) {
	rows, err := c.db.Query(`SELECT project_id, name, kind, url_map, backend_service, updated_at
		FROM target_proxies ORDER BY project_id, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTargetProxies(rows)
}

func (c *Cache) GetTargetProxy(projectID, name string) (*TargetProxy, error) {
	rows, err := c.db.Query(`SELECT project_id, name, kind, url_map, backend_service, updated_at
		FROM target_proxies WHERE project_id = ? AND name = ?`, projectID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	proxies, err := scanTargetProxies(rows)
	if err != nil {
		return nil, err
	}
	if len(proxies) == 0 {
		return nil, nil
	}
	return &proxies[0], nil
}

func (c *Cache) SaveURLMaps(projectID string, maps []URLMap) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM url_maps WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, m := range maps {
		pathRules, err := json.Marshal(m.PathRules)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO url_maps
			(project_id, name, default_service, path_rules, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			projectID, m.Name, m.DefaultService, string(pathRules), updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanURLMaps(rows *sql.Rows) ([]URLMap, error) {
	var maps []URLMap
	for rows.Next() {
		var m URLMap
		var pathRules string
		if err := rows.Scan(&m.ProjectID, &m.Name, &m.DefaultService, &pathRules, &m.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(pathRules), &m.PathRules); err != nil {
			return nil, err
		}
		maps = append(maps, m)
	}
	return maps, rows.Err()
}

func (c *Cache) ListURLMaps(projectID string) ([]URLMap, error) {
	rows, err := c.db.Query(`SELECT project_id, name, default_service, path_rules, updated_at
		FROM url_maps WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanURLMaps(rows)
}

func (c *Cache) ListAllURLMaps() ([]URLMap, error) {
	rows, err := c.db.Query(`SELECT project_id, name, default_service, path_rules, updated_at
		FROM url_maps ORDER BY project_id, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanURLMaps(rows)
}

func (c *Cache) GetURLMap(projectID, name string) (*URLMap, error) {
	rows, err := c.db.Query(`SELECT project_id, name, default_service, path_rules, updated_at
		FROM url_maps WHERE project_id = ? AND name = ?`, projectID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	maps, err := scanURLMaps(rows)
	if err != nil {
		return nil, err
	}
	if len(maps) == 0 {
		return nil, nil
	}
	return &maps[0], nil
}

func (c *Cache) SaveBackendServices(projectID string, services []BackendService) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM backend_services WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, s := range services {
		backends, err := json.Marshal(s.Backends)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO backend_services
			(project_id, region, name, protocol, port_name, backends, health_checks, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, s.Region, s.Name, s.Protocol, s.PortName, string(backends), strings.Join(s.HealthChecks, ","), updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanBackendServices(rows *sql.Rows) ([]BackendService, error) {
	var services []BackendService
	for rows.Next() {
		var s BackendService
		var backends, healthChecks string
		if err := rows.Scan(&s.ProjectID, &s.Region, &s.Name, &s.Protocol, &s.PortName, &backends, &healthChecks, &s.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(backends), &s.Backends); err != nil {
			return nil, err
		}
		if healthChecks != "" {
			s.HealthChecks = strings.Split(healthChecks, ",")
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

func (c *Cache) ListBackendServices(projectID string) ([]BackendService, error) {
	rows, err := c.db.Query(`SELECT project_id, region, name, protocol, port_name, backends, health_checks, updated_at
		FROM backend_services WHERE project_id = ? ORDER BY region, name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBackendServices(rows)
}

func (c *Cache) ListAllBackendServices() ([]BackendService, error) {
	rows, err := c.db.Query(`SELECT project_id, region, name, protocol, port_name, backends, health_checks, updated_at
		FROM backend_services ORDER BY project_id, region, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBackendServices(rows)
}

func (c *Cache) GetBackendService(projectID, region, name string) (*BackendService, error) {
	rows, err := c.db.Query(`SELECT project_id, region, name, protocol, port_name, backends, health_checks, updated_at
		FROM backend_services WHERE project_id = ? AND region = ? AND name = ?`, projectID, region, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	services, err := scanBackendServices(rows)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return nil, nil
	}
	return &services[0], nil
}

type LoadBalancer struct {
	Rule            ForwardingRule
	Proxy           *TargetProxy
	URLMap          *URLMap
	BackendServices []BackendService
}

func (c *Cache) ResolveLoadBalancer(projectID, region, name string) (*LoadBalancer, error) {
	rule, err := c.GetForwardingRule(projectID, region, name)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, nil
	}
	lb := &LoadBalancer{Rule: *rule}
	var serviceNames []string
	switch rule.TargetKind {
	case "backendServices":
		serviceNames = append(serviceNames, rule.Target)
	case "targetHttpProxies", "targetHttpsProxies", "targetTcpProxies", "targetSslProxies", "targetGrpcProxies":
		proxy, err := c.GetTargetProxy(projectID, rule.Target)
		if err != nil {
			return nil, err
		}
		if proxy != nil {
			lb.Proxy = proxy
			switch {
			case proxy.BackendService != "":
				serviceNames = append(serviceNames, proxy.BackendService)
			case proxy.URLMap != "":
				urlMap, err := c.GetURLMap(projectID, proxy.URLMap)
				if err != nil {
					return nil, err
				}
				if urlMap != nil {
					lb.URLMap = urlMap
					serviceNames = append(serviceNames, urlMap.DefaultService)
					for _, pr := range urlMap.PathRules {
						if pr.Service != "" && !slices.Contains(serviceNames, pr.Service) {
							serviceNames = append(serviceNames, pr.Service)
						}
					}
				}
			}
		}
	}
	regions := []string{region}
	if region != "global" {
		regions = append(regions, "global")
	}
	for _, serviceName := range serviceNames {
		if serviceName == "" {
			continue
		}
		svc, err := c.getBackendServiceAnyRegion(projectID, regions, serviceName)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			lb.BackendServices = append(lb.BackendServices, *svc)
		}
	}
	return lb, nil
}

func (c *Cache) getBackendServiceAnyRegion(projectID string, regions []string, name string) (*BackendService, error) {
	for _, region := range regions {
		svc, err := c.GetBackendService(projectID, region, name)
		if err != nil {
			return nil, err
		}
		if svc != nil {
			return svc, nil
		}
	}
	return nil, nil
}

func (c *Cache) SaveClusters(projectID string, clusters []Cluster) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM clusters WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, cl := range clusters {
		nodePools, err := json.Marshal(cl.NodePools)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO clusters
			(project_id, name, location, description, status, version, endpoint, node_pools, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, cl.Name, cl.Location, cl.Description, cl.Status, cl.Version, cl.Endpoint, string(nodePools), cl.CreatedAt, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanClusters(rows *sql.Rows) ([]Cluster, error) {
	var clusters []Cluster
	for rows.Next() {
		var cl Cluster
		var nodePools string
		if err := rows.Scan(&cl.ProjectID, &cl.Name, &cl.Location, &cl.Description, &cl.Status, &cl.Version, &cl.Endpoint, &nodePools, &cl.CreatedAt, &cl.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(nodePools), &cl.NodePools); err != nil {
			return nil, err
		}
		clusters = append(clusters, cl)
	}
	return clusters, rows.Err()
}

func (c *Cache) ListClusters(projectID string) ([]Cluster, error) {
	rows, err := c.db.Query(`SELECT project_id, name, location, description, status, version, endpoint, node_pools, created_at, updated_at
		FROM clusters WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClusters(rows)
}

func (c *Cache) ListAllClusters() ([]Cluster, error) {
	rows, err := c.db.Query(`SELECT project_id, name, location, description, status, version, endpoint, node_pools, created_at, updated_at
		FROM clusters ORDER BY project_id, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClusters(rows)
}

func (c *Cache) GetCluster(projectID, name string) (*Cluster, error) {
	rows, err := c.db.Query(`SELECT project_id, name, location, description, status, version, endpoint, node_pools, created_at, updated_at
		FROM clusters WHERE project_id = ? AND name = ?`, projectID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	clusters, err := scanClusters(rows)
	if err != nil {
		return nil, err
	}
	if len(clusters) == 0 {
		return nil, nil
	}
	return &clusters[0], nil
}

func (c *Cache) SaveWorkloads(projectID, cluster string, workloads []Workload) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM workloads WHERE project_id = ? AND cluster = ?", projectID, cluster); err != nil {
		return err
	}
	for _, w := range workloads {
		if _, err := tx.Exec(`INSERT INTO workloads
			(project_id, cluster, kind, namespace, name, replicas, images, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, cluster, w.Kind, w.Namespace, w.Name, w.Replicas, strings.Join(w.Images, ","), updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanWorkloads(rows *sql.Rows) ([]Workload, error) {
	var workloads []Workload
	for rows.Next() {
		var w Workload
		var images string
		if err := rows.Scan(&w.ProjectID, &w.Cluster, &w.Kind, &w.Namespace, &w.Name, &w.Replicas, &images, &w.UpdatedAt); err != nil {
			return nil, err
		}
		if images != "" {
			w.Images = strings.Split(images, ",")
		}
		workloads = append(workloads, w)
	}
	return workloads, rows.Err()
}

func (c *Cache) ListClusterWorkloads(projectID, cluster string) ([]Workload, error) {
	rows, err := c.db.Query(`SELECT project_id, cluster, kind, namespace, name, replicas, images, updated_at
		FROM workloads WHERE project_id = ? AND cluster = ? ORDER BY namespace, kind, name`, projectID, cluster)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkloads(rows)
}

func (c *Cache) GetWorkload(projectID, cluster, kind, namespace, name string) (*Workload, error) {
	rows, err := c.db.Query(`SELECT project_id, cluster, kind, namespace, name, replicas, images, updated_at
		FROM workloads WHERE project_id = ? AND cluster = ? AND kind = ? AND namespace = ? AND name = ?`,
		projectID, cluster, kind, namespace, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	workloads, err := scanWorkloads(rows)
	if err != nil {
		return nil, err
	}
	if len(workloads) == 0 {
		return nil, nil
	}
	return &workloads[0], nil
}

func (c *Cache) Close() error {
	return c.db.Close()
}

func (c *Cache) SaveDNSZones(projectID string, zones []DNSZone) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM dns_zones WHERE project_id = ?", projectID); err != nil {
		return err
	}
	for _, z := range zones {
		if _, err := tx.Exec(`INSERT INTO dns_zones
			(project_id, name, dns_name, visibility, description, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			projectID, z.Name, z.DNSName, z.Visibility, z.Description, updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanDNSZones(rows *sql.Rows) ([]DNSZone, error) {
	defer rows.Close()
	var zones []DNSZone
	for rows.Next() {
		var z DNSZone
		if err := rows.Scan(&z.ProjectID, &z.Name, &z.DNSName, &z.Visibility, &z.Description, &z.UpdatedAt); err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

func (c *Cache) ListDNSZones(projectID string) ([]DNSZone, error) {
	rows, err := c.db.Query(`SELECT project_id, name, dns_name, visibility, description, updated_at
		FROM dns_zones WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	return scanDNSZones(rows)
}

func (c *Cache) ListAllDNSZones() ([]DNSZone, error) {
	rows, err := c.db.Query(`SELECT project_id, name, dns_name, visibility, description, updated_at
		FROM dns_zones ORDER BY project_id, name`)
	if err != nil {
		return nil, err
	}
	return scanDNSZones(rows)
}

func (c *Cache) GetDNSZone(projectID, name string) (*DNSZone, error) {
	rows, err := c.db.Query(`SELECT project_id, name, dns_name, visibility, description, updated_at
		FROM dns_zones WHERE project_id = ? AND name = ?`, projectID, name)
	if err != nil {
		return nil, err
	}
	zones, err := scanDNSZones(rows)
	if err != nil {
		return nil, err
	}
	if len(zones) == 0 {
		return nil, nil
	}
	return &zones[0], nil
}

func (c *Cache) SaveDNSRecords(projectID, zone string, records []DNSRecord) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec("DELETE FROM dns_records WHERE project_id = ? AND zone = ?", projectID, zone); err != nil {
		return err
	}
	for _, r := range records {
		if _, err := tx.Exec(`INSERT INTO dns_records
			(project_id, zone, name, type, ttl, data, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			projectID, zone, r.Name, r.Type, r.TTL, strings.Join(r.Data, ","), updatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanDNSRecords(rows *sql.Rows) ([]DNSRecord, error) {
	defer rows.Close()
	var records []DNSRecord
	for rows.Next() {
		var r DNSRecord
		var data string
		if err := rows.Scan(&r.ProjectID, &r.Zone, &r.Name, &r.Type, &r.TTL, &data, &r.UpdatedAt); err != nil {
			return nil, err
		}
		if data != "" {
			r.Data = strings.Split(data, ",")
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (c *Cache) ListZoneDNSRecords(projectID, zone string) ([]DNSRecord, error) {
	rows, err := c.db.Query(`SELECT project_id, zone, name, type, ttl, data, updated_at
		FROM dns_records WHERE project_id = ? AND zone = ? ORDER BY name, type`, projectID, zone)
	if err != nil {
		return nil, err
	}
	return scanDNSRecords(rows)
}

func (c *Cache) GetDNSRecord(projectID, zone, name, recordType string) (*DNSRecord, error) {
	rows, err := c.db.Query(`SELECT project_id, zone, name, type, ttl, data, updated_at
		FROM dns_records WHERE project_id = ? AND zone = ? AND name = ? AND type = ?`,
		projectID, zone, name, recordType)
	if err != nil {
		return nil, err
	}
	records, err := scanDNSRecords(rows)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}
