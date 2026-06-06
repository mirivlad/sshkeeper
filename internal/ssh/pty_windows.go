//go:build windows

package ssh

import "fmt"

const windowsPTYUnsupportedMessage = "password and key-passphrase auth are not supported on Windows experimental builds; use key or agent auth with OpenSSH Client"

func ConnectWithPassword(sshBinary string, args []string, password string) error {
	return fmt.Errorf("%s", windowsPTYUnsupportedMessage)
}

func connectWithPasswordAndRead(sshBinary string, args []string, password string, timeoutSec int) (bool, string) {
	return false, windowsPTYUnsupportedMessage
}
