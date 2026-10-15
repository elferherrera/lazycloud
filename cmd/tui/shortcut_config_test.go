package main

import "testing"

func numericShortcut(key rune) *shortcut {
	for i := range shortcuts {
		if shortcuts[i].key == key {
			return &shortcuts[i]
		}
	}
	return nil
}

func TestApplyShortcutConfigDefaults(t *testing.T) {
	applyShortcutConfig(map[string]string{})
	if s := numericShortcut('1'); s == nil || s.desc != "show Projects" {
		t.Fatalf("key 1 = %+v, want show Projects", s)
	}
	if s := numericShortcut('0'); s == nil || s.desc != "show Firewall Rules" {
		t.Fatalf("key 0 = %+v, want show Firewall Rules", s)
	}
}

func TestApplyShortcutConfigOverridesAndInvalidEntries(t *testing.T) {
	defer applyShortcutConfig(defaultShortcutConfig())

	applyShortcutConfig(map[string]string{"1": "dns-zones", "5": "no-such-view", "x": "vms"})
	if s := numericShortcut('1'); s == nil || s.desc != "show DNS Zones" {
		t.Fatalf("key 1 = %+v, want show DNS Zones", s)
	}
	if s := numericShortcut('5'); s == nil || s.desc != "show IP Addresses" {
		t.Fatalf("key 5 = %+v, want default kept for unknown view", s)
	}
	if s := numericShortcut('2'); s == nil || s.desc != "show VMs" {
		t.Fatalf("key 2 = %+v, want default kept for unconfigured key", s)
	}
}