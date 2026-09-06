package tui

import (
	"testing"

	sessionpkg "github.com/mirivlad/sshkeeper/internal/session"
)

func TestSessionCloseCommandMarksOperationComplete(t *testing.T) {
	model := newSessionScreenModel(80, 24)
	model.setSessions([]sessionpkg.Window{{ID: "@sshkeeper-test-missing", ServerAlias: "test"}})

	cmd := model.closeSelected()
	if cmd == nil {
		t.Fatal("closeSelected returned nil command")
	}
	msg, ok := cmd().(sessionsLoadedMsg)
	if !ok {
		t.Fatalf("closeSelected returned %T, want sessionsLoadedMsg", cmd())
	}
	if !msg.closed {
		t.Fatal("closeSelected did not mark the close operation complete")
	}
}
