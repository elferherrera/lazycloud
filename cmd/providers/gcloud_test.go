package providers

import (
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
)

func strptr(s string) *string { return &s }

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
