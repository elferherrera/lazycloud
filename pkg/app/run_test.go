package app

import "testing"

func TestCheckNestedExecution(t *testing.T) {
	t.Setenv("LAZYCLOUD_CONTEXT", "true")
	err := checkNestedExecution()
	if err == nil || err.Error() != "Detected nested lazycloud execution. Exit this context to start lazycloud" {
		t.Errorf("checkNestedExecution() = %v, want nested execution error", err)
	}
	t.Setenv("LAZYCLOUD_CONTEXT", "false")
	if err := checkNestedExecution(); err != nil {
		t.Errorf("checkNestedExecution() = %v, want nil for false", err)
	}
	t.Setenv("LAZYCLOUD_CONTEXT", "")
	if err := checkNestedExecution(); err != nil {
		t.Errorf("checkNestedExecution() = %v, want nil for empty", err)
	}
}
