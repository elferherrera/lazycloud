package providers

// Shared resource model: the shapes the cache stores and both cloud
// adapters map their API objects onto. Keeping them here (not inside
// any provider package) is what lets the cache and the CLI stay
// cloud-agnostic.

type Project struct {
	ProjectID     string
	Name          string
	ProjectNumber string
}

type Network struct {
	ProjectID             string
	Name                  string
	Description           string
	RoutingMode           string
	AutoCreateSubnetworks bool
	MTU                   int32
	UpdatedAt             string
}

type Subnetwork struct {
	ProjectID             string
	Name                  string
	Region                string
	Network               string
	IPRange               string
	SecondaryRanges       []string
	PrivateIPGoogleAccess bool
	Purpose               string
	Description           string
	Status                string
	UpdatedAt             string
}

type VMInterface struct {
	Network    string
	Subnetwork string
	InternalIP string
	ExternalIP string
}

type VMDisk struct {
	Name       string
	DeviceName string
	Mode       string
	AutoDelete bool
	SizeGB     int64
}

type VMServiceAccount struct {
	Email  string
	Scopes []string
}

type VM struct {
	ProjectID        string
	Name             string
	InstanceID       string
	Zone             string
	MachineType      string
	Status           string
	Labels           map[string]string
	NetworkTags      []string
	ServiceAccounts  []VMServiceAccount
	InstanceTemplate string
	Interfaces       []VMInterface
	BootDisk         *VMDisk
	DataDisks        []VMDisk
	UpdatedAt        string
}

type Disk struct {
	ProjectID   string
	Name        string
	Zone        string
	SizeGB      int64
	Type        string
	Status      string
	SourceImage string
	Users       []string
	UpdatedAt   string
}

type ServiceAccount struct {
	ProjectID   string
	Email       string
	DisplayName string
	UniqueID    string
	Description string
	Disabled    bool
	UpdatedAt   string
}

type Bucket struct {
	ProjectID     string
	Name          string
	Location      string
	StorageClass  string
	Versioning    bool
	UniformAccess bool
	CreatedAt     string
	UpdatedAt     string
}

type ClusterNodePool struct {
	Name      string
	Version   string
	NodeCount int64
}

type Cluster struct {
	ProjectID   string
	Name        string
	Description string
	Location    string
	Status      string
	Version     string
	Endpoint    string
	Labels      map[string]string
	NodePools   []ClusterNodePool
	CreatedAt   string
	UpdatedAt   string
}

type IAMBinding struct {
	ProjectID string
	Member    string
	Role      string
	Condition string
	UpdatedAt string
}

type IPAddress struct {
	ProjectID   string
	Name        string
	Address     string
	AddressType string
	Status      string
	Region      string
	Subnetwork  string
	Network     string
	UpdatedAt   string
}

type NetworkPeering struct {
	ProjectID            string
	Network              string
	Name                 string
	PeerNetwork          string
	PeerProject          string
	State                string
	StateDetails         string
	AutoCreateRoutes     bool
	ExchangeSubnetRoutes bool
	ExportCustomRoutes   bool
	ImportCustomRoutes   bool
	UpdatedAt            string
}

type Snapshot struct {
	ProjectID       string
	Name            string
	Description     string
	Status          string
	SourceDisk      string
	SizeGB          int64
	StorageBytes    int64
	CreatedAt       string
	StorageLocation string
	Labels          map[string]string
	UpdatedAt       string
}

type Route struct {
	ProjectID   string
	Name        string
	Network     string
	DestRange   string
	Priority    int64
	NextHopKind string
	NextHop     string
	Description string
	UpdatedAt   string
}

type FirewallRule struct {
	ProjectID   string
	Name        string
	Network     string
	Direction   string
	Priority    int64
	Action      string
	Source      string
	Targets     string
	Disabled    bool
	Description string
	UpdatedAt   string
}

type ForwardingRule struct {
	ProjectID  string
	Name       string
	Region     string
	IPAddress  string
	PortRange  string
	Protocol   string
	Scheme     string
	TargetKind string
	Target     string
	TargetURL  string
	UpdatedAt  string
}

type TargetProxy struct {
	ProjectID      string
	Kind           string
	Name           string
	URLMap         string
	BackendService string
	UpdatedAt      string
}

type PathRule struct {
	Paths   []string
	Service string
}

type URLMap struct {
	ProjectID      string
	Name           string
	DefaultService string
	PathRules      []PathRule
	UpdatedAt      string
}

type Backend struct {
	Group         string
	BalancingMode string
}

type BackendService struct {
	ProjectID    string
	Name         string
	Region       string
	Protocol     string
	PortName     string
	Backends     []Backend
	HealthChecks []string
	UpdatedAt    string
}

type PSCConnection struct {
	Endpoint string
	Project  string
	Network  string
	Status   string
}

type ServiceAttachment struct {
	ProjectID     string
	Name          string
	Region        string
	TargetService string
	Preference    string
	Description   string
	Connections   []PSCConnection
	UpdatedAt     string
}

type Workload struct {
	ProjectID string
	Cluster   string
	Kind      string
	Namespace string
	Name      string
	Replicas  string
	Images    []string
	UpdatedAt string
}

type ClusterNode struct {
	ProjectID  string
	Cluster    string
	Name       string
	NodePool   string
	Status     string
	InternalIP string
	UpdatedAt  string
}

type DNSZone struct {
	ProjectID   string
	Name        string
	DNSName     string
	Visibility  string
	Description string
	UpdatedAt   string
}

type DNSRecord struct {
	ProjectID string
	Zone      string
	Name      string
	Type      string
	TTL       int64
	Data      []string
	UpdatedAt string
}

type CloudRouterPeer struct {
	Name      string
	Interface string
	IPAddress string
	PeerIP    string
	PeerASN   uint32
	Priority  uint32
}

type CloudRouter struct {
	ProjectID   string
	Name        string
	Region      string
	Network     string
	Description string
	BGPASN      uint32
	Peers       []CloudRouterPeer
	NATs        []string
	CreatedAt   string
	UpdatedAt   string
}

type VPNTunnel struct {
	Name      string
	Interface int32
	PeerIP    string
	IKE       int32
	Status    string
	Router    string
}

type VPNGateway struct {
	ProjectID   string
	Name        string
	Type        string
	Region      string
	Network     string
	IPs         []string
	Description string
	Tunnels     []VPNTunnel
	CreatedAt   string
	UpdatedAt   string
}

type InstanceGroup struct {
	ProjectID   string
	Name        string
	Zone        string
	Type        string
	Size        int32
	Network     string
	Subnetwork  string
	Template    string
	Description string
	UpdatedAt   string
}

type InstanceGroupMember struct {
	Zone string
	Name string
}

type Reservation struct {
	ProjectID   string
	Name        string
	Zone        string
	MachineType string
	Count       int64
	InUseCount  int64
	ShareType   string
	Required    bool
	Status      string
	Description string
	Commitment  string
	UpdatedAt   string
}

type Image struct {
	ProjectID       string
	Name            string
	Family          string
	Status          string
	SizeGB          int64
	StorageLocation string
	Source          string
	CreatedAt       string
	Description     string
	Deprecated      string
	UpdatedAt       string
}

type InstanceTemplateDisk struct {
	DeviceName  string
	SourceImage string
	DiskSizeGB  int64
	DiskType    string
	AutoDelete  bool
	Boot        bool
}

type InstanceTemplateInterface struct {
	Network    string
	Subnetwork string
	NetworkIP  string
	ExternalIP string
}

type InstanceTemplate struct {
	ProjectID   string
	Name        string
	Location    string
	MachineType string
	Disks       []InstanceTemplateDisk
	Interfaces  []InstanceTemplateInterface
	Labels      map[string]string
	Description string
	CreatedAt   string
	UpdatedAt   string
}

type BucketIAMBinding struct {
	ProjectID string
	Bucket    string
	Member    string
	Role      string
	Condition string
	UpdatedAt string
}

type CustomRole struct {
	ProjectID   string
	Name        string
	Title       string
	Description string
	Stage       string
	Permissions []string
	Deleted     bool
	UpdatedAt   string
}

var PublicImageProjects = []string{
	"almalinux-cloud",
	"centos-cloud",
	"cos-cloud",
	"debian-cloud",
	"opensuse-cloud",
	"oracle-linux-cloud",
	"rhel-cloud",
	"rocky-linux-cloud",
	"suse-cloud",
	"suse-sap-cloud",
	"ubuntu-os-cloud",
	"windows-cloud",
	"windows-sql-cloud",
}
