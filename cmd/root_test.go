package cmd

import "testing"

func TestCommandRequiresStartupVaultUnlock(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "root tui", args: nil, want: true},
		{name: "connect", args: []string{"connect", "prod"}, want: true},
		{name: "short connect alias", args: []string{"c", "prod"}, want: true},
		{name: "add can store secrets", args: []string{"add", "prod"}, want: true},
		{name: "vault handles its own unlock", args: []string{"vault", "list"}, want: false},
		{name: "list only reads database", args: []string{"list"}, want: false},
		{name: "show only reads database", args: []string{"show", "prod"}, want: false},
		{name: "search only reads database", args: []string{"search", "prod"}, want: false},
		{name: "config path only reads config", args: []string{"config", "path"}, want: false},
		{name: "help", args: []string{"--help"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandRequiresStartupVaultUnlock(tt.args); got != tt.want {
				t.Fatalf("commandRequiresStartupVaultUnlock(%v) = %v; want %v", tt.args, got, tt.want)
			}
		})
	}
}
