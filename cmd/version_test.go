package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	original := Version
	Version = "v9.8.7-test"
	t.Cleanup(func() { Version = original })

	var out bytes.Buffer
	cmd := newVersionCmd()
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("version command: %v", err)
	}
	if got, want := out.String(), "sshkeeper v9.8.7-test\n"; got != want {
		t.Fatalf("output = %q; want %q", got, want)
	}
}

func TestRootVersionFlagIsEnabled(t *testing.T) {
	if rootCmd.Version == "" {
		t.Fatal("root command Version must be set so Cobra exposes --version")
	}
	if !strings.Contains(rootCmd.Version, "v") && rootCmd.Version != "dev" {
		t.Fatalf("unexpected root version %q", rootCmd.Version)
	}
}
