package providers

import (
	"encoding/json"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"
)

func strptr(s string) *string { return &s }

func TestInstanceRefFromLink(t *testing.T) {
	link := "https://www.googleapis.com/compute/v1/projects/revolut-labs/zones/europe-west2-c/instances/guacamole-nfnz"
	zone, name := instanceRefFromLink(link)
	if zone != "europe-west2-c" || name != "guacamole-nfnz" {
		t.Fatalf("instanceRefFromLink() = %q, %q, want europe-west2-c, guacamole-nfnz", zone, name)
	}

	zone, name = instanceRefFromLink("projects/p/regions/europe-west2/instances/vm-a")
	if zone != "europe-west2" || name != "vm-a" {
		t.Fatalf("instanceRefFromLink() region = %q, %q, want europe-west2, vm-a", zone, name)
	}

	zone, name = instanceRefFromLink("")
	if zone != "" || name != "" {
		t.Fatalf("instanceRefFromLink() empty = %q, %q, want empty", zone, name)
	}
}

func TestForwardingRuleFrom(t *testing.T) {
	rule := forwardingRuleFrom("p1", "global", &computepb.ForwardingRule{
		Name:                strptr("fr-web"),
		IPAddress:           strptr("34.1.1.1"),
		PortRange:           strptr("443"),
		IPProtocol:          strptr("TCP"),
		LoadBalancingScheme: strptr("EXTERNAL"),
		Target:              strptr("https://www.googleapis.com/compute/v1/projects/p1/global/targetHttpsProxies/proxy-a"),
		BackendService:      strptr("https://www.googleapis.com/compute/v1/projects/p1/regions/europe-west2/backendServices/bs-ignored"),
	})
	if rule.Name != "fr-web" || rule.Region != "global" || rule.IPAddress != "34.1.1.1" || rule.PortRange != "443" || rule.Protocol != "TCP" || rule.Scheme != "EXTERNAL" {
		t.Fatalf("forwardingRuleFrom() = %+v", rule)
	}
	if rule.TargetKind != "targetHttpsProxies" || rule.Target != "proxy-a" {
		t.Fatalf("forwardingRuleFrom() target = %q (%q), want proxy-a via targetHttpsProxies", rule.Target, rule.TargetKind)
	}

	rule = forwardingRuleFrom("p1", "europe-west2", &computepb.ForwardingRule{
		Name:                strptr("fwr-lb-internal-vpn-gateway-forward-tcp-all"),
		IPAddress:           strptr("10.201.97.253"),
		IPProtocol:          strptr("TCP"),
		LoadBalancingScheme: strptr("INTERNAL"),
		BackendService:      strptr("https://www.googleapis.com/compute/v1/projects/p1/regions/europe-west2/backendServices/vpn-gateway-forward"),
	})
	if rule.TargetKind != "backendServices" || rule.Target != "vpn-gateway-forward" {
		t.Fatalf("forwardingRuleFrom() target = %q (%q), want backend service taken from the backendService field", rule.Target, rule.TargetKind)
	}

	rule = forwardingRuleFrom("p1", "global", &computepb.ForwardingRule{Name: strptr("fr-bare")})
	if rule.TargetKind != "" || rule.Target != "" {
		t.Fatalf("forwardingRuleFrom() target = %q (%q), want empty when both fields are unset", rule.Target, rule.TargetKind)
	}
}

func TestInstanceToVMMapsEveryField(t *testing.T) {
	inst := &computepb.Instance{
		Name:        strptr("web-1"),
		Id:          proto.Uint64(12345),
		Zone:        strptr("projects/p1/zones/europe-west2-b"),
		MachineType: strptr("projects/p1/zones/europe-west2-b/machineTypes/e2-small"),
		Status:      strptr("RUNNING"),
		Labels:      map[string]string{"env": "prod"},
		Tags:        &computepb.Tags{Items: []string{"ssh"}},
		ServiceAccounts: []*computepb.ServiceAccount{
			{Email: strptr("vm@p1.iam.gserviceaccount.com"), Scopes: []string{"cloud-platform"}},
		},
		Metadata: &computepb.Metadata{
			Items: []*computepb.Items{
				{Key: strptr("instance-template"), Value: strptr("projects/p1/global/instanceTemplates/tpl-1")},
			},
		},
		NetworkInterfaces: []*computepb.NetworkInterface{
			{
				Network:    strptr("projects/p1/global/networks/default"),
				Subnetwork: strptr("projects/p1/regions/europe-west2/subnetworks/main"),
				NetworkIP:  strptr("10.0.0.5"),
				AccessConfigs: []*computepb.AccessConfig{
					{NatIP: strptr("")},
					{NatIP: strptr("34.10.10.10")},
				},
			},
		},
		Disks: []*computepb.AttachedDisk{
			{Boot: proto.Bool(true), Source: strptr("projects/p1/zones/europe-west2-b/disks/boot"), DeviceName: strptr("boot"), Mode: strptr("READ_WRITE"), DiskSizeGb: proto.Int64(20)},
			{Boot: proto.Bool(false), Source: strptr("projects/p1/zones/europe-west2-b/disks/data"), DeviceName: strptr("data"), Mode: strptr("READ_WRITE"), DiskSizeGb: proto.Int64(100)},
		},
	}

	vm := instanceToVM("p1", inst)

	if vm.ProjectID != "p1" || vm.Name != "web-1" || vm.InstanceID != "12345" {
		t.Fatalf("identity = %q %q %q", vm.ProjectID, vm.Name, vm.InstanceID)
	}
	if vm.Zone != "europe-west2-b" {
		t.Fatalf("zone = %q, want short name", vm.Zone)
	}
	if vm.MachineType != "e2-small" {
		t.Fatalf("machine type = %q, want short name", vm.MachineType)
	}
	if vm.Status != "RUNNING" || vm.Labels["env"] != "prod" || vm.NetworkTags[0] != "ssh" {
		t.Fatalf("status/labels/tags = %q %v %v", vm.Status, vm.Labels, vm.NetworkTags)
	}
	if len(vm.ServiceAccounts) != 1 || vm.ServiceAccounts[0].Email != "vm@p1.iam.gserviceaccount.com" {
		t.Fatalf("service accounts = %+v", vm.ServiceAccounts)
	}
	if vm.InstanceTemplate != "tpl-1" {
		t.Fatalf("instance template = %q, want short name", vm.InstanceTemplate)
	}
	if len(vm.Interfaces) != 1 {
		t.Fatalf("interfaces = %d, want 1", len(vm.Interfaces))
	}
	iface := vm.Interfaces[0]
	if iface.Network != "default" || iface.Subnetwork != "main" || iface.InternalIP != "10.0.0.5" {
		t.Fatalf("interface = %+v", iface)
	}
	// The empty NAT IP comes first; mapping must keep scanning for a real one.
	if iface.ExternalIP != "34.10.10.10" {
		t.Fatalf("external IP = %q, want first non-empty NAT IP", iface.ExternalIP)
	}
	if vm.BootDisk == nil || vm.BootDisk.Name != "boot" {
		t.Fatalf("boot disk = %+v", vm.BootDisk)
	}
	if len(vm.DataDisks) != 1 || vm.DataDisks[0].Name != "data" {
		t.Fatalf("data disks = %+v", vm.DataDisks)
	}
}

func TestInstanceToVMWithoutOptionalFields(t *testing.T) {
	vm := instanceToVM("p1", &computepb.Instance{Name: strptr("bare")})
	if vm.BootDisk != nil || len(vm.DataDisks) != 0 || len(vm.Interfaces) != 0 {
		t.Fatalf("bare instance produced disks/interfaces: %+v", vm)
	}
	if vm.InstanceTemplate != "" {
		t.Fatalf("instance template = %q, want empty", vm.InstanceTemplate)
	}
}

func TestZoneVMsSkipsInstancesWithoutZone(t *testing.T) {
	inst := &computepb.Instance{
		Name:   strptr("no-zone"),
		Status: strptr("RUNNING"),
	}
	vms := zoneVMs("p1", []*computepb.Instance{inst})
	if len(vms) != 0 {
		t.Fatalf("zoneVMs() = %d vms, want 0 for zone-less instance", len(vms))
	}
}

func TestZoneVMsMapsInstances(t *testing.T) {
	inst := &computepb.Instance{
		Name:   strptr("vm1"),
		Zone:   strptr("projects/p1/zones/us-east1-b"),
		Status: strptr("RUNNING"),
	}
	vms := zoneVMs("p1", []*computepb.Instance{inst})
	if len(vms) != 1 {
		t.Fatalf("zoneVMs() = %d vms, want 1", len(vms))
	}
	if vms[0].Zone != "us-east1-b" || vms[0].ProjectID != "p1" {
		t.Fatalf("zoneVMs() = %+v, want zone short name and project", vms[0])
	}
}

func TestParseClusterNodesMapsStatusAndAddresses(t *testing.T) {
	raw := `{"items":[
		{
			"metadata": {"name": "node-a", "labels": {"cloud.google.com/gke-nodepool": "pool1"}},
			"status": {
				"conditions": [{"type": "Ready", "status": "True"}],
				"addresses": [{"type": "InternalIP", "address": "10.0.0.1"}]
			}
		},
		{
			"metadata": {"name": "node-b"},
			"status": {
				"conditions": [{"type": "Ready", "status": "False"}, {"type": "DiskPressure", "status": "True"}]
			}
		}
	]}`
	var list k8sNodeList
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatalf("unmarshal() error = %v", err)
	}
	nodes := parseClusterNodes("p1", "prod", list.Items)
	if len(nodes) != 2 {
		t.Fatalf("parseClusterNodes() = %d nodes, want 2", len(nodes))
	}
	a := nodes[0]
	if a.Name != "node-a" || a.NodePool != "pool1" || a.Status != "Ready" || a.InternalIP != "10.0.0.1" {
		t.Fatalf("ready node = %+v", a)
	}
	if nodes[1].Status != "NotReady" || nodes[1].InternalIP != "" {
		t.Fatalf("not-ready node = %+v", nodes[1])
	}
}
