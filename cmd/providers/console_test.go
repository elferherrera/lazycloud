package providers

import "testing"

func TestGCPVMLogsURL(t *testing.T) {
	got := GCPVMLogsURL("proj-1", "1234567890", "vm-a")
	want := "https://console.cloud.google.com/logs/query;query=resource.type%3D%22gce_instance%22%0Aresource.labels.instance_id%3D%221234567890%22%0AjsonPayload.hostname%3D%22vm-a%22?project=proj-1"
	if got != want {
		t.Fatalf("GCPVMLogsURL() =\n%s\nwant\n%s", got, want)
	}

	got = GCPVMLogsURL("proj-1", "", "vm-a")
	want = "https://console.cloud.google.com/logs/query;query=resource.type%3D%22gce_instance%22%0AjsonPayload.hostname%3D%22vm-a%22?project=proj-1"
	if got != want {
		t.Fatalf("GCPVMLogsURL() fallback =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPWorkloadLogsURL(t *testing.T) {
	got := GCPWorkloadLogsURL("revolut-dev-apps", "europe-west2", "apps-dev-europe-west2", "alert-courier", "alert-courier")
	want := "https://console.cloud.google.com/logs/query;query=resource.type%3D%22k8s_container%22%0Aresource.labels.project_id%3D%22revolut-dev-apps%22%0Aresource.labels.location%3D%22europe-west2%22%0Aresource.labels.cluster_name%3D%22apps-dev-europe-west2%22%0Aresource.labels.namespace_name%3D%22alert-courier%22%0Aseverity%3E%3DDEFAULT;storageScope=project;cursorTimestamp=;duration=PT30M?project=revolut-dev-apps"
	if got != want {
		t.Fatalf("GCPWorkloadLogsURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPWorkloadOverviewURL(t *testing.T) {
	got := GCPWorkloadOverviewURL("revolut-dev-apps", "alert-courier")
	want := "https://console.cloud.google.com/kubernetes/workload/overview?project=revolut-dev-apps&pageState=(%22savedViews%22:(%22n%22:%5B%22alert-courier%22%5D))"
	if got != want {
		t.Fatalf("GCPWorkloadOverviewURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPClusterURL(t *testing.T) {
	got := GCPClusterURL("proj-1", "europe-west1", "cluster-a")
	want := "https://console.cloud.google.com/kubernetes/clusters/details/europe-west1/cluster-a?project=proj-1"
	if got != want {
		t.Fatalf("GCPClusterURL() = %q, want %q", got, want)
	}
}

func TestGCPClusterNodePoolURL(t *testing.T) {
	got := GCPClusterNodePoolURL("revolut-prod-apps", "europe-west2", "apps-prod-europe-west2", "default-pool")
	want := "https://console.cloud.google.com/kubernetes/nodepool/europe-west2/apps-prod-europe-west2/default-pool?project=revolut-prod-apps"
	if got != want {
		t.Fatalf("GCPClusterNodePoolURL() = %q, want %q", got, want)
	}
}

func TestGCPClusterEventsURL(t *testing.T) {
	got := GCPClusterEventsURL("revolut-prod-apps", "apps-prod-europe-west2")
	want := "https://console.cloud.google.com/logs/query;query=" +
		"log_id%28%22events%22%29" +
		"%0Aresource.type%3D%22k8s_cluster%22" +
		"%0Aresource.labels.cluster_name%3D%22apps-prod-europe-west2%22" +
		"%0Aresource.labels.project_id%3D%22revolut-prod-apps%22" +
		";storageScope=project;cursorTimestamp=;duration=PT30M?project=revolut-prod-apps"
	if got != want {
		t.Fatalf("GCPClusterNodeEventsURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPWorkloadEventsURL(t *testing.T) {
	got := GCPWorkloadEventsURL("revolut-prod-apps", "europe-west2", "apps-prod-europe-west2", "web")
	want := "https://console.cloud.google.com/logs/query;query=" +
		"log_id%28%22events%22%29" +
		"%0Aresource.type%3D%22k8s_pod%22" +
		"%0Aresource.labels.cluster_name%3D%22apps-prod-europe-west2%22" +
		"%0Aresource.labels.location%3D%22europe-west2%22" +
		"%0Aresource.labels.project_id%3D%22revolut-prod-apps%22" +
		"%0A%22web%22" +
		";storageScope=project;cursorTimestamp=;duration=PT30M?project=revolut-prod-apps"
	if got != want {
		t.Fatalf("GCPWorkloadEventsURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPNodePoolEventsURL(t *testing.T) {
	got := GCPNodePoolEventsURL("revolut-labs", "integration", "default-pool")
	want := "https://console.cloud.google.com/logs/query;query=" +
		"log_id%28%22events%22%29" +
		"%0Aresource.type%3D%22k8s_node%22" +
		"%0Aresource.labels.cluster_name%3D%22integration%22" +
		"%0Aresource.labels.project_id%3D%22revolut-labs%22" +
		"%0A%22default-pool%22" +
		";storageScope=project;cursorTimestamp=;duration=PT30M?project=revolut-labs"
	if got != want {
		t.Fatalf("GCPNodePoolEventsURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPLoadBalancerURL(t *testing.T) {
	got := GCPLoadBalancerURL("revolut-dev-apps", "alert-courier")
	want := "https://console.cloud.google.com/net-services/loadbalancing/details/http/alert-courier?project=revolut-dev-apps"
	if got != want {
		t.Fatalf("GCPLoadBalancerURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPLoadBalancerFrontendURL(t *testing.T) {
	got := GCPLoadBalancerFrontendURL("revolut-dev-apps", "-forward-")
	want := "https://console.cloud.google.com/net-services/loadbalancing/list/frontends?project=revolut-dev-apps&pageState=(%22frontends%22:(%22f%22:%22%255B%257B_22k_22_3A_22_22_2C_22t_22_3A10_2C_22v_22_3A_22_5C_22-forward-_5C_22_22%257D%255D%22))"
	if got != want {
		t.Fatalf("GCPLoadBalancerFrontendURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPAddressesURL(t *testing.T) {
	got := GCPAddressesURL("revolut-dev-apps", "dns-forwarding-2bab1b111322b008")
	want := "https://console.cloud.google.com/networking/addresses/list?referrer=search&project=revolut-dev-apps&pageState=(%22allAddressesTable%22:(%22f%22:%22%255B%257B_22k_22_3A_22name_22_2C_22t_22_3A10_2C_22v_22_3A_22_5C_22dns-forwarding-2bab1b111322b008_5C_22_22_2C_22i_22_3A_22name_22%257D%255D%22))"
	if got != want {
		t.Fatalf("GCPAddressesURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPRoutesURL(t *testing.T) {
	got := GCPRoutesURL("revolut-dev-apps", "default-route")
	want := "https://console.cloud.google.com/networking/routes/details/default-route?project=revolut-dev-apps"
	if got != want {
		t.Fatalf("GCPRoutesURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPFirewallsURL(t *testing.T) {
	got := GCPFirewallsURL("revolut-dev-apps", "allow-https")
	want := "https://console.cloud.google.com/net-security/firewall-manager/firewall-policies/details/allow-https?project=revolut-dev-apps"
	if got != want {
		t.Fatalf("GCPFirewallsURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPCloudRouterURL(t *testing.T) {
	got := GCPCloudRouterURL("revolut-labs", "europe-west2", "router-vpns-net-nebius-dev-europe-west2")
	want := "https://console.cloud.google.com/hybrid/routers/details/europe-west2/router-vpns-net-nebius-dev-europe-west2?project=revolut-labs"
	if got != want {
		t.Fatalf("GCPCloudRouterURL() =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPVPNGatewayURL(t *testing.T) {
	got := GCPVPNGatewayURL("revolut-labs", "europe-west2", "ha-vpn-gw-dev-ai-nbs-euw2", true)
	want := "https://console.cloud.google.com/hybrid/vpn/gateways/details/europe-west2/ha-vpn-gw-dev-ai-nbs-euw2?project=revolut-labs&isHA=true"
	if got != want {
		t.Fatalf("GCPVPNGatewayURL() =\n%s\nwant\n%s", got, want)
	}

	got = GCPVPNGatewayURL("revolut-labs", "europe-west2", "gw-classic", false)
	want = "https://console.cloud.google.com/hybrid/vpn/gateways/details/europe-west2/gw-classic?project=revolut-labs&isHA=false"
	if got != want {
		t.Fatalf("GCPVPNGatewayURL() classic =\n%s\nwant\n%s", got, want)
	}
}

func TestGCPInstanceGroupURL(t *testing.T) {
	got := GCPInstanceGroupURL("revolut-labs", "europe-west2-b", "vpn-gateway-labs-nebius-prod")
	want := "https://console.cloud.google.com/compute/instanceGroups/details/europe-west2-b/vpn-gateway-labs-nebius-prod?project=revolut-labs"
	if got != want {
		t.Fatalf("GCPInstanceGroupURL() =\n%s\nwant\n%s", got, want)
	}

	got = GCPInstanceGroupURL("revolut-labs", "europe-west2", "guacamole")
	want = "https://console.cloud.google.com/compute/instanceGroups/details/europe-west2/guacamole?project=revolut-labs"
	if got != want {
		t.Fatalf("GCPInstanceGroupURL() regional =\n%s\nwant\n%s", got, want)
	}
}
