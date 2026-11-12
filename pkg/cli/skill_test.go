package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillDeps returns deps whose skill seams point at a temp dir, so
// no test ever touches the real home directory.
func skillDeps(t *testing.T) (deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	base := t.TempDir()
	return deps{
		skillsBaseDir: func() (string, error) { return filepath.Join(base, "skills"), nil },
		installSkill:  installSkillFiles,
	}, new(bytes.Buffer), new(bytes.Buffer)
}

func TestSkillInstallDefault(t *testing.T) {
	d, stdout, stderr := skillDeps(t)
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d, want 0; stderr: %s", code, stderr.String())
	}
	target := strings.TrimSpace(stdout.String())
	if filepath.Base(target) != skillFileName {
		t.Fatalf("printed path = %q, want .../%s", target, skillFileName)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading installed skill: %v", err)
	}
	if !strings.Contains(string(got), "name: lazycloud") {
		t.Error("installed skill missing frontmatter name")
	}
}

func TestSkillInstallOverwrites(t *testing.T) {
	d, stdout, stderr := skillDeps(t)
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("first install: %d; stderr: %s", code, stderr.String())
	}
	// Second install must replace, not fail.
	stdout.Reset()
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install"}, stdout, stderr, d); code != exitOK {
		t.Fatalf("second install: %d; stderr: %s", code, stderr.String())
	}
}

func TestSkillInstallPathFlag(t *testing.T) {
	base := t.TempDir()
	d, stdout, stderr := skillDeps(t)
	args := []string{"lazycloud", "skill", "install", "--path", base}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(base, skillName, skillFileName)); err != nil {
		t.Fatalf("skill not at --path target: %v", err)
	}
}

func TestSkillInstallPathEqualsForm(t *testing.T) {
	base := t.TempDir()
	d, stdout, stderr := skillDeps(t)
	args := []string{"lazycloud", "skill", "install", "--path=" + base}
	if code := ExecuteDeps(args, stdout, stderr, d); code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(base, skillName, skillFileName)); err != nil {
		t.Fatalf("skill not at --path= target: %v", err)
	}
}

func TestSkillInstallUnknownSubcommand(t *testing.T) {
	d, stdout, stderr := skillDeps(t)
	code := ExecuteDeps([]string{"lazycloud", "skill", "remove"}, stdout, stderr, d)
	if code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("stderr = %q, want usage line", stderr.String())
	}
}

func TestSkillInstallMissingSubcommand(t *testing.T) {
	d, stdout, stderr := skillDeps(t)
	if code := ExecuteDeps([]string{"lazycloud", "skill"}, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestSkillInstallPathMissingValue(t *testing.T) {
	d, stdout, stderr := skillDeps(t)
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install", "--path"}, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestSkillInstallUnknownFlag(t *testing.T) {
	d, stdout, stderr := skillDeps(t)
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install", "--force"}, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unknown flag "--force"`) {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestSkillInstallUnexpectedArgument(t *testing.T) {
	d, stdout, stderr := skillDeps(t)
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install", "extra"}, stdout, stderr, d); code != exitUsage {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestSkillInstallBaseDirError(t *testing.T) {
	d, stdout, stderr := skillDeps(t)
	d.skillsBaseDir = func() (string, error) { return "", errors.New("no home") }
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install"}, stdout, stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "resolving skills dir") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestSkillInstallWriteError(t *testing.T) {
	d, stdout, stderr := skillDeps(t)
	d.installSkill = func(_, _, _ string) (string, error) { return "", errors.New("disk full") }
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install"}, stdout, stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "installing skill") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestSkillInstallMkdirError(t *testing.T) {
	// A file squatting on the target dir name makes MkdirAll fail.
	file := filepath.Join(t.TempDir(), "lazycloud")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, stdout, stderr := skillDeps(t)
	d.skillsBaseDir = func() (string, error) { return filepath.Dir(file), nil }
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install"}, stdout, stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
}

func TestSkillInstallWriteFileError(t *testing.T) {
	// A directory squatting on the SKILL.md path makes WriteFile fail.
	base := t.TempDir()
	target := filepath.Join(base, skillName, skillFileName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	d, stdout, stderr := skillDeps(t)
	d.skillsBaseDir = func() (string, error) { return base, nil }
	if code := ExecuteDeps([]string{"lazycloud", "skill", "install"}, stdout, stderr, d); code != exitError {
		t.Fatalf("exit = %d, want 3", code)
	}
}

func TestDefaultSkillsBaseDirEnv(t *testing.T) {
	t.Setenv("LAZYCLOUD_SKILLS_DIR", "/tmp/skills-xyz")
	got, err := defaultSkillsBaseDir()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "/tmp/skills-xyz" {
		t.Errorf("base dir = %q, want env value", got)
	}
}

func TestDefaultSkillsBaseDirHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LAZYCLOUD_SKILLS_DIR", "")
	got, err := defaultSkillsBaseDir()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != filepath.Join(home, ".agents", "skills") {
		t.Errorf("base dir = %q, want %s/.agents/skills", got, home)
	}
}

func TestDefaultSkillsBaseDirNoHome(t *testing.T) {
	// An empty HOME makes os.UserHomeDir fail on unix; that error
	// must reach the caller as an exit-3 install failure.
	t.Setenv("HOME", "")
	t.Setenv("LAZYCLOUD_SKILLS_DIR", "")
	if _, err := defaultSkillsBaseDir(); err == nil {
		t.Fatal("empty HOME must fail, want error")
	}
}

func TestSkillContentCoversCLIContract(t *testing.T) {
	c := skillContent()
	for _, want := range []string{"name: lazycloud", "Exit codes: 0 ok", "Selectors", "recipes"} {
		if !strings.Contains(c, want) {
			t.Errorf("skill content missing %q", want)
		}
	}
}

func TestSkillContentCoversNebius(t *testing.T) {
	c := skillContent()
	for _, want := range []string{"Nebius setup", "NEBIUS_IAM_TOKEN", "--cloud gcp|nebius", "cache-nebius.db", "group:<name>"} {
		if !strings.Contains(c, want) {
			t.Errorf("skill content missing nebius fact %q", want)
		}
	}
}

// TestSkillDescriptionTriggersOnCloudSearch pins the trigger words: an
// agent asked to search for resources in GCP, Google Cloud, or Nebius must
// match the description, in the vocabularies users actually use.
func TestSkillDescriptionTriggersOnCloudSearch(t *testing.T) {
	c := skillContent()
	for _, want := range []string{"search", "Google Cloud", "GCP", "Nebius", "cloud resources"} {
		if !strings.Contains(c, want) {
			t.Errorf("skill description missing trigger %q", want)
		}
	}
}
