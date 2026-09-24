package i18n

import "testing"

func TestLanguageSelection(t *testing.T) {
	previous := Preference()
	t.Cleanup(func() { _ = SetPreference(previous) })
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "ru_RU.UTF-8")
	if err := SetPreference(Auto); err != nil {
		t.Fatal(err)
	}
	if Effective() != Russian || T("Forward", "Проброс") != "Проброс" {
		t.Fatal("Russian system locale was not selected")
	}
	t.Setenv("LC_MESSAGES", "en_GB.UTF-8")
	if Effective() != English {
		t.Fatal("LC_MESSAGES did not override LANG")
	}
	t.Setenv("LC_ALL", "ru-RU")
	if Effective() != Russian {
		t.Fatal("LC_ALL did not override LC_MESSAGES")
	}
	if err := SetPreference(English); err != nil {
		t.Fatal(err)
	}
	if Effective() != English || Tf("Port %d", "Порт %d", 3389) != "Port 3389" {
		t.Fatal("manual English override was not applied")
	}
	if err := SetPreference("de"); err == nil || Preference() != English {
		t.Fatal("invalid language preference was accepted")
	}
}
