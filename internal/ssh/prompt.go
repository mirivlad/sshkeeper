package ssh

import "regexp"

// passwordPromptRe matches the password and key-passphrase prompts of
// OpenSSH, so the stored secret is typed exactly when ssh asks for it.
var passwordPromptRe = regexp.MustCompile(`(?i)(password|passphrase).*:\s*$`)
