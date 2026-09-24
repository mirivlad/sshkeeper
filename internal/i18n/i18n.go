package i18n

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
)

const (
	Auto    = "auto"
	Russian = "ru"
	English = "en"
)

var preference atomic.Value

func init() {
	preference.Store(Auto)
}

func ValidPreference(value string) bool {
	return value == Auto || value == Russian || value == English
}

func SetPreference(value string) error {
	if !ValidPreference(value) {
		return fmt.Errorf("unsupported language preference %q", value)
	}
	preference.Store(value)
	return nil
}

func Preference() string {
	return preference.Load().(string)
}

// Effective returns the selected language. An unsupported system locale uses
// English, so an untranslated locale never results in mixed UI languages.
func Effective() string {
	if selected := Preference(); selected != Auto {
		return selected
	}
	return SystemLanguage()
}

func SystemLanguage() string {
	if runtime.GOOS == "windows" {
		return languageOf(platformLocale())
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if locale := os.Getenv(name); locale != "" {
			return languageOf(locale)
		}
	}
	return languageOf(platformLocale())
}

func languageOf(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == Russian || strings.HasPrefix(locale, "ru_") || strings.HasPrefix(locale, "ru-") || strings.HasPrefix(locale, "ru.") {
		return Russian
	}
	return English
}

func T(english, russian string) string {
	if Effective() == Russian {
		return russian
	}
	return english
}

func Tf(english, russian string, args ...any) string {
	return fmt.Sprintf(T(english, russian), args...)
}
