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

func GCPBucketURL(projectID, name string) string {
	return "https://console.cloud.google.com/storage/browser/" +
		url.PathEscape(name) + ";tab=objects?project=" + url.QueryEscape(projectID)
}

func gcpLBListURL(projectID, table, name string) string {
	filter := fmt.Sprintf(`[{"k":"","t":10,"v":"\"%s\""}]`, name)
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
	pageState := `(%22` + table + `%22:(%22f%22:%22` + encoded + `%22))`
	return "https://console.cloud.google.com/net-services/loadbalancing/list/" + table +
		"?project=" + url.QueryEscape(projectID) + "&pageState=" + pageState
}

func GCPWorkloadLogsURL(projectID, location, clusterName, namespace, name string) string {
	var sb strings.Builder
	sb.WriteString("resource.type=\"k8s_container\"\n")
	sb.WriteString(fmt.Sprintf("resource.labels.project_id=\"%s\"\n", projectID))
	sb.WriteString(fmt.Sprintf("resource.labels.location=\"%s\"\n", location))
	sb.WriteString(fmt.Sprintf("resource.labels.cluster_name=\"%s\"\n", clusterName))
	sb.WriteString(fmt.Sprintf("resource.labels.namespace_name=\"%s\"\n", namespace))
	sb.WriteString("severity>=DEFAULT")
	encoded := strings.ReplaceAll(url.QueryEscape(sb.String()), "+", "%20")
	return "https://console.cloud.google.com/logs/query;query=" + encoded +
		";storageScope=project;cursorTimestamp=;duration=PT30M?project=" + url.QueryEscape(projectID)
}

func GCPWorkloadOverviewURL(projectID, namespace string) string {
	return "https://console.cloud.google.com/kubernetes/workload/overview?project=" +
		url.QueryEscape(projectID) +
		"&pageState=(%22savedViews%22:(%22n%22:%5B%22" + namespace + "%22%5D))"
}

func GCPWorkloadEventsURL(projectID, location, clusterName, workloadName string) string {
	var sb strings.Builder
	sb.WriteString("log_id(\"events\")\n")
	sb.WriteString("resource.type=\"k8s_pod\"\n")
	sb.WriteString(fmt.Sprintf("resource.labels.cluster_name=\"%s\"\n", clusterName))
	sb.WriteString(fmt.Sprintf("resource.labels.location=\"%s\"\n", location))
	sb.WriteString(fmt.Sprintf("resource.labels.project_id=\"%s\"\n", projectID))
	sb.WriteString(fmt.Sprintf("%q", workloadName))
	encoded := strings.ReplaceAll(url.QueryEscape(sb.String()), "+", "%20")
	return "https://console.cloud.google.com/logs/query;query=" + encoded +
		";storageScope=project;cursorTimestamp=;duration=PT30M?project=" + url.QueryEscape(projectID)
}

func GCPClusterEventsURL(projectID, clusterName string) string {
	var sb strings.Builder
	sb.WriteString("log_id(\"events\")\n")
	sb.WriteString("resource.type=\"k8s_cluster\"\n")
	sb.WriteString(fmt.Sprintf("resource.labels.cluster_name=\"%s\"\n", clusterName))
	sb.WriteString(fmt.Sprintf("resource.labels.project_id=\"%s\"", projectID))
	encoded := strings.ReplaceAll(url.QueryEscape(sb.String()), "+", "%20")
	return "https://console.cloud.google.com/logs/query;query=" + encoded +
		";storageScope=project;cursorTimestamp=;duration=PT30M?project=" + url.QueryEscape(projectID)
}

func GCPNodePoolEventsURL(projectID, clusterName, poolName string) string {
	var sb strings.Builder
	sb.WriteString("log_id(\"events\")\n")
	sb.WriteString("resource.type=\"k8s_node\"\n")
	sb.WriteString(fmt.Sprintf("resource.labels.cluster_name=\"%s\"\n", clusterName))
	sb.WriteString(fmt.Sprintf("resource.labels.project_id=\"%s\"\n", projectID))
	sb.WriteString(fmt.Sprintf("%q", poolName))
	encoded := strings.ReplaceAll(url.QueryEscape(sb.String()), "+", "%20")
	return "https://console.cloud.google.com/logs/query;query=" + encoded +
		";storageScope=project;cursorTimestamp=;duration=PT30M?project=" + url.QueryEscape(projectID)
}

func GCPClusterURL(projectID, location, name string) string {
	return "https://console.cloud.google.com/kubernetes/clusters/details/" +
		url.PathEscape(location) + "/" + url.PathEscape(name) + "?project=" + url.QueryEscape(projectID)
}

func GCPClusterNodePoolURL(projectID, location, cluster, pool string) string {
	return "https://console.cloud.google.com/kubernetes/nodepool/" +
		url.PathEscape(location) + "/" + url.PathEscape(cluster) + "/" + url.PathEscape(pool) + "?project=" + url.QueryEscape(projectID)
}

func GCPLoadBalancerURL(projectID, urlMapName string) string {
	return "https://console.cloud.google.com/net-services/loadbalancing/details/http/" +
		url.PathEscape(urlMapName) + "?project=" + url.QueryEscape(projectID)
}

func GCPLoadBalancerFrontendURL(projectID, forwardingRuleName string) string {
	return gcpLBListURL(projectID, "frontends", forwardingRuleName)
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

func GCPRoutesURL(projectID, name string) string {
	return "https://console.cloud.google.com/networking/routes/details/" +
		url.PathEscape(name) + "?project=" + url.QueryEscape(projectID)
}

func GCPFirewallsURL(projectID, name string) string {
	return "https://console.cloud.google.com/net-security/firewall-manager/firewall-policies/details/" +
		url.PathEscape(name) + "?project=" + url.QueryEscape(projectID)
}

func GCPDNSZoneURL(projectID, zone string) string {
	return "https://console.cloud.google.com/net-services/dns/zones/" +
		url.PathEscape(zone) + "/records?project=" + url.QueryEscape(projectID)
}

func GCPCloudRouterURL(projectID, region, name string) string {
	return "https://console.cloud.google.com/hybrid/routers/details/" +
		url.PathEscape(region) + "/" + url.PathEscape(name) + "?project=" + url.QueryEscape(projectID)
}

func GCPVPNGatewayURL(projectID, region, name string, isHA bool) string {
	isHAParam := "false"
	if isHA {
		isHAParam = "true"
	}
	return "https://console.cloud.google.com/hybrid/vpn/gateways/details/" +
		url.PathEscape(region) + "/" + url.PathEscape(name) +
		"?project=" + url.QueryEscape(projectID) + "&isHA=" + isHAParam
}
