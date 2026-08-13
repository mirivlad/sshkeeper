package tui

import (
	"fmt"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/model"
)

func TestForwardFormDigitsReachFocusedInput(t *testing.T) {
	fm := newForwardFormModel(1, 100, 30)
	fm.focusIdx = 2 + len(forwardTypes) + 1
	fm.updateFocus()

	for _, digit := range []rune{'1', '2', '3'} {
		updated, _ := fm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{digit}})
		fm = updated.(*forwardFormModel)
	}

	if got := fm.inputs[1].Value(); got != "123" {
		t.Fatalf("listen port = %q, want %q", got, "123")
	}
	if fm.currentType != model.ForwardLocal {
		t.Fatalf("forward type = %q, want %q", fm.currentType, model.ForwardLocal)
	}
}

func TestRemoteForwardFormMapsListenAndTargetEndpoints(t *testing.T) {
	oldSave := SaveForward
	t.Cleanup(func() { SaveForward = oldSave })
	var saved *model.Forward
	SaveForward = func(forward *model.Forward) error {
		copy := *forward
		saved = &copy
		return nil
	}

	fm := newForwardFormModel(7, 80, 24)
	fm.currentType = model.ForwardRemote
	fm.typeIdx = typeIndex(model.ForwardRemote)
	fm.nameInput.SetValue("remote web")
	fm.inputs[0].SetValue("0.0.0.0")
	fm.inputs[1].SetValue("18080")
	fm.inputs[2].SetValue("127.0.0.1")
	fm.inputs[3].SetValue("8080")
	msg := fm.runSave()()
	if result, ok := msg.(saveDoneMsg); !ok || result.err != nil {
		t.Fatalf("save result = %#v", msg)
	}
	if saved == nil {
		t.Fatal("forward was not saved")
	}
	if saved.RemoteAddr != "0.0.0.0" || saved.RemotePort != 18080 || saved.LocalAddr != "127.0.0.1" || saved.LocalPort != 8080 {
		t.Fatalf("remote forward endpoints were reversed: %#v", saved)
	}
	wantArgs := []string{"-R", "0.0.0.0:18080:127.0.0.1:8080"}
	if got := saved.ForwardSSHArgs(); !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("ForwardSSHArgs() = %#v, want %#v", got, wantArgs)
	}
}

func TestRemoteForwardEditPopulatesSemanticFields(t *testing.T) {
	forward := &model.Forward{ID: 9, Type: model.ForwardRemote, RemoteAddr: "0.0.0.0", RemotePort: 18080, LocalAddr: "127.0.0.1", LocalPort: 8080}
	fm := newForwardEditModel(7, forward, 80, 24)
	want := []string{"0.0.0.0", "18080", "127.0.0.1", "8080"}
	for index, value := range want {
		if got := fm.inputs[index].Value(); got != value {
			t.Fatalf("input[%d] = %q, want %q", index, got, value)
		}
	}
}

func TestForwardFormDigitShortcutsWorkOnTypeSelector(t *testing.T) {
	tests := []struct {
		digit rune
		want  model.ForwardType
		idx   int
	}{
		{digit: '1', want: model.ForwardLocal, idx: 0},
		{digit: '2', want: model.ForwardRemote, idx: 1},
		{digit: '3', want: model.ForwardDynamic, idx: 2},
	}

	for focusIdx := 2; focusIdx < 2+len(forwardTypes); focusIdx++ {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("focus_%d_digit_%c", focusIdx, tt.digit), func(t *testing.T) {
				fm := newForwardFormModel(1, 100, 30)
				fm.currentType = model.ForwardDynamic
				fm.typeIdx = typeIndex(model.ForwardDynamic)
				if tt.want == model.ForwardDynamic {
					fm.currentType = model.ForwardLocal
					fm.typeIdx = typeIndex(model.ForwardLocal)
				}
				fm.focusIdx = focusIdx
				fm.updateFocus()

				updated, _ := fm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.digit}})
				fm = updated.(*forwardFormModel)

				if fm.currentType != tt.want {
					t.Fatalf("forward type = %q, want %q", fm.currentType, tt.want)
				}
				if fm.typeIdx != tt.idx {
					t.Fatalf("type index = %d, want %d", fm.typeIdx, tt.idx)
				}
			})
		}
	}
}
