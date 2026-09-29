package tui

import (
	"os"
	"testing"
	"time"

	"github.com/mirivlad/sshkeeper/internal/i18n"
)

// TestMain pins the interface language to English and shortens timers. Most assertions check
// English labels, and the default "auto" preference would otherwise follow the
// developer's locale and fail on a Russian workstation.
func TestMain(m *testing.M) {
	_ = i18n.SetPreference(i18n.English)
	// Notice expiry ticks run inside batched commands; keep them instant.
	noticeLifetime = time.Millisecond
	os.Exit(m.Run())
}
