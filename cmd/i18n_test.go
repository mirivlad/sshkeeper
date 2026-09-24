package cmd

import (
	"strings"
	"testing"

	"github.com/mirivlad/sshkeeper/internal/i18n"
)

func TestCLIEnglishAndRussianMessages(t *testing.T) {
	previous := i18n.Preference()
	t.Cleanup(func() {
		_ = i18n.SetPreference(previous)
		localizeCLIHelp()
	})

	for _, tc := range []struct {
		language   string
		help       string
		status     string
		flag       string
		usage      string
		completion string
	}{
		{i18n.English, "List all servers", "Vault: locked (vault commands unlock per command)", "Server hostname or IP", "Available Commands:", "Generate the autocompletion script"},
		{i18n.Russian, "Показать все серверы", "Хранилище: заблокировано (разблокировка отдельно для каждой команды)", "Имя хоста или IP-адрес сервера", "Доступные команды:", "Создать скрипт автодополнения"},
	} {
		if err := i18n.SetPreference(tc.language); err != nil {
			t.Fatal(err)
		}
		localizeCLIHelp()
		if listCmd.Short != tc.help {
			t.Errorf("%s help = %q; want %q", tc.language, listCmd.Short, tc.help)
		}
		if got := formatVaultStatus(false, true); got != tc.status {
			t.Errorf("%s status = %q; want %q", tc.language, got, tc.status)
		}
		if got := addCmd.Flags().Lookup("host").Usage; !strings.Contains(got, tc.flag) {
			t.Errorf("%s flag help = %q; want %q", tc.language, got, tc.flag)
		}
		if !strings.Contains(rootCmd.UsageTemplate(), tc.usage) {
			t.Errorf("%s usage template lacks %q", tc.language, tc.usage)
		}
		for _, command := range rootCmd.Commands() {
			if command.Name() == "completion" && !strings.Contains(command.Short, tc.completion) {
				t.Errorf("%s completion help = %q; want %q", tc.language, command.Short, tc.completion)
			}
		}
	}
}
