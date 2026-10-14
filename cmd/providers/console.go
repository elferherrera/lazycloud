package providers

import (
	"fmt"
	"net/url"
	"strings"
)

func GCPNetworkURL(projectID, name string) string {
	return "https://console.cloud.google.com/networking/networks/details/" +
		url.PathEscape(name) + "?project=" + url.QueryEscape(projectID)
}

func GCPSubnetworkURL(projectID, region, name string) string {
	return "https://console.cloud.google.com/networking/subnetworks/details/" +
		url.PathEscape(region) + "/" + url.PathEscape(name) + "?project=" + url.QueryEscape(projectID)
}

func GCPServiceAccountURL(projectID, email string) string {
	return "https://console.cloud.google.com/iam-admin/serviceaccounts/details/" +
		url.PathEscape(email) + "?project=" + url.QueryEscape(projectID)
}

func GCPVMURL(projectID, zone, name string) string {
	return "https://console.cloud.google.com/compute/instancesDetail/zones/" +
		url.PathEscape(zone) + "/instances/" + url.PathEscape(name) + "?project=" + url.QueryEscape(projectID)
}

func GCPVMLogsURL(projectID, instanceID, name string) string {
	var sb strings.Builder
	sb.WriteString("resource.type=\"gce_instance\"\n")
	if instanceID != "" {
		sb.WriteString(fmt.Sprintf("resource.labels.instance_id=\"%s\"\n", instanceID))
	}
	if name != "" {
		sb.WriteString(fmt.Sprintf("jsonPayload.hostname=\"%s\"", name))
	}
	encoded := strings.ReplaceAll(url.QueryEscape(strings.TrimRight(sb.String(), "\n")), "+", "%20")
	return "https://console.cloud.google.com/logs/query;query=" + encoded + "?project=" + url.QueryEscape(projectID)
}

func GCPAddressesURL(projectID, name string) string {
	filter := fmt.Sprintf(`[{"k":"name","t":10,"v":"\"%s\"","i":"name"}]`, name)
	encoded := strings.NewReplacer(
		`\`, `_5C`,
		`"`, `_22`,
		`:`, `_3A`,
		`,`, `_2C`,
	).Replace(filter)
	encoded = strings.NewReplacer(
		`[`, `%5B`,
		`{`, `%7B`,
		`}`, `%7D`,
		`]`, `%5D`,
	).Replace(encoded)
	encoded = strings.ReplaceAll(encoded, "%", "%25")
	pageState := `(%22allAddressesTable%22:(%22f%22:%22` + encoded + `%22))`
	return "https://console.cloud.google.com/networking/addresses/list?referrer=search&project=" +
		url.QueryEscape(projectID) + "&pageState=" + pageState
}
