package app

import (
	"strings"
	"testing"
)

func TestKubectlSetContextArgs(t *testing.T) {
	got := strings.Join(kubectlSetContextArgs("cluster-1", "prod"), " ")
	if got != "config set-context cluster-1 --namespace prod" {
		t.Errorf("kubectlSetContextArgs() = %q, want %q", got, "config set-context cluster-1 --namespace prod")
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
