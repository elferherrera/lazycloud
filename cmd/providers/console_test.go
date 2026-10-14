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
	want := "https://console.cloud.google.com/logs/query;query=resource.type%3D%22k8s_container%22%0Aresource.labels.project_id%3D%22revolut-dev-apps%22%0Aresource.labels.location%3D%22europe-west2%22%0Aresource.labels.cluster_name%3D%22apps-dev-europe-west2%22%0Aresource.labels.namespace_name%3D%22alert-courier%22%0Alabels.k8s-pod%2Fapp%3D%22alert-courier%22%20severity%3E%3DDEFAULT;storageScope=project;cursorTimestamp=;duration=PT30M?project=revolut-dev-apps"
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

func TestGCPLoadBalancerURL(t *testing.T) {
	got := GCPLoadBalancerURL("revolut-dev-apps", "alert-courier")
	want := "https://console.cloud.google.com/net-services/loadbalancing/list/loadBalancers?project=revolut-dev-apps&pageState=(%22loadBalancers%22:(%22f%22:%22%255B%257B_22k_22_3A_22_22_2C_22t_22_3A10_2C_22v_22_3A_22_5C_22alert-courier_5C_22_22%257D%255D%22))"
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
