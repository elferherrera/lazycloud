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

func TestGCPAddressesURL(t *testing.T) {
	got := GCPAddressesURL("revolut-dev-apps", "dns-forwarding-2bab1b111322b008")
	want := "https://console.cloud.google.com/networking/addresses/list?referrer=search&project=revolut-dev-apps&pageState=(%22allAddressesTable%22:(%22f%22:%22%255B%257B_22k_22_3A_22name_22_2C_22t_22_3A10_2C_22v_22_3A_22_5C_22dns-forwarding-2bab1b111322b008_5C_22_22_2C_22i_22_3A_22name_22%257D%255D%22))"
	if got != want {
		t.Fatalf("GCPAddressesURL() =\n%s\nwant\n%s", got, want)
	}
}
