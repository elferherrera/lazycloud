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

func TestKubectlUseContextArgs(t *testing.T) {
	got := strings.Join(kubectlUseContextArgs("gke_my-prod_europe-west2-a_api"), " ")
	if got != "config use-context gke_my-prod_europe-west2-a_api" {
		t.Errorf("kubectlUseContextArgs() = %q, want %q", got, "config use-context gke_my-prod_europe-west2-a_api")
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

func TestWorkloadShellCommandEnv(t *testing.T) {
	cmd := workloadShellCommand()
	found := false
	for _, kv := range cmd.Env {
		if kv == "LAZYCLOUD_CONTEXT=true" {
			found = true
		}
	}
	if !found {
		t.Errorf("workloadShellCommand() env = %v, want LAZYCLOUD_CONTEXT=true", cmd.Env)
	}
}

func TestZellijRunArgs(t *testing.T) {
	got := strings.Join(zellijRunArgs("prod", "Deployment", "api-gateway"), " ")
	if got != "run --floating -- k9s --namespace prod --command dp /api-gateway" {
		t.Errorf("zellijRunArgs() = %q, want %q", got, "run --floating -- k9s --namespace prod --command dp /api-gateway")
	}
}

func TestK9sCommand(t *testing.T) {
	if got := k9sCommand("Deployment", "api-gateway"); got != "dp /api-gateway" {
		t.Errorf("k9sCommand(Deployment, api-gateway) = %q, want %q", got, "dp /api-gateway")
	}
	if got := k9sCommand("Pod", ""); got != "po" {
		t.Errorf("k9sCommand(Pod, empty) = %q, want %q", got, "po")
	}
}

func TestK9sViewForKind(t *testing.T) {
	cases := map[string]string{
		"Deployment":  "dp",
		"StatefulSet": "sts",
		"DaemonSet":   "ds",
		"ReplicaSet":  "rs",
		"Pod":         "po",
		"Job":         "po",
	}
	for kind, want := range cases {
		if got := k9sViewForKind(kind); got != want {
			t.Errorf("k9sViewForKind(%q) = %q, want %q", kind, got, want)
		}
	}
}
