package app

import (
	"strings"
	"testing"
)

func TestKubectlSetContextArgs(t *testing.T) {
	got := strings.Join(kubectlSetContextArgs("gke_my-prod_europe-west2-a_api", "prod"), " ")
	if got != "config set-context gke_my-prod_europe-west2-a_api --namespace prod" {
		t.Errorf("kubectlSetContextArgs() = %q, want %q", got, "config set-context gke_my-prod_europe-west2-a_api --namespace prod")
	}
}

func TestFindClusterContextName(t *testing.T) {
	names := []string{
		"default",
		"gke_other-prod_europe-west2-b_api",
		"gke_my-prod_europe-west2-a_api",
		"gke_my-prod_europe-west2-a_api-v2",
	}
	if got := findClusterContextName(names, "my-prod", "europe-west2-a", "api"); got != "gke_my-prod_europe-west2-a_api" {
		t.Errorf("exact match: findClusterContextName() = %q, want %q", got, "gke_my-prod_europe-west2-a_api")
	}
	if got := findClusterContextName(names, "my-prod", "europe-west1", "api"); got != "gke_my-prod_europe-west2-a_api" {
		t.Errorf("suffix match preferring project: findClusterContextName() = %q, want %q", got, "gke_my-prod_europe-west2-a_api")
	}
	if got := findClusterContextName(names, "third-prod", "europe-west1", "api"); got != "gke_other-prod_europe-west2-b_api" {
		t.Errorf("suffix match without project: findClusterContextName() = %q, want %q", got, "gke_other-prod_europe-west2-b_api")
	}
	if got := findClusterContextName([]string{"my-api-cluster"}, "", "", "api"); got != "my-api-cluster" {
		t.Errorf("contains fallback: findClusterContextName() = %q, want %q", got, "my-api-cluster")
	}
	if got := findClusterContextName(names, "my-prod", "europe-west2-a", "missing"); got != "" {
		t.Errorf("no match: findClusterContextName() = %q, want empty", got)
	}
}

func TestShellPath(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	if got := shellPath(); got != "/bin/zsh" {
		t.Errorf("shellPath() = %q, want /bin/zsh", got)
	}
	t.Setenv("SHELL", "")
	if got := shellPath(); got != "/bin/sh" {
		t.Errorf("shellPath() = %q, want /bin/sh", got)
	}
}
