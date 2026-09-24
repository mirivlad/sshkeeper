package cmd

import "github.com/mirivlad/sshkeeper/internal/i18n"

func tr(en, ru string) string { return i18n.T(en, ru) }

func trf(en, ru string, args ...any) string { return i18n.Tf(en, ru, args...) }
