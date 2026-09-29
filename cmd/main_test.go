package cmd

import (
	"os"
	"testing"

	"github.com/mirivlad/sshkeeper/internal/i18n"
)

// TestMain pins the interface language to English. Most assertions check
// English labels, and the default "auto" preference would otherwise follow the
// developer's locale and fail on a Russian workstation.
func TestMain(m *testing.M) {
	_ = i18n.SetPreference(i18n.English)
	os.Exit(m.Run())
}
