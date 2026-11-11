//go:build windows

package tunnel

import "fmt"

// Windows OpenSSH uses the system ssh-agent service rather than Unix sockets.
// Keep the secret-bearing background path disabled until it has a tested
// Windows-specific implementation; key/agent tunnels remain supported.
func supportsPassphraseBackground() bool { return false }

func startPassphraseAgent(identityFile, passphrase string) (*passphraseAgent, error) {
	return nil, fmt.Errorf("background key+passphrase tunnels are not supported on Windows; use a foreground tunnel or load the key into the Windows ssh-agent")
}

func stopPassphraseAgent(pid int, dir string) {}
