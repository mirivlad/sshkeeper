# sshkeeper

`sshkeeper` is a Linux console manager for SSH profiles, secrets, and quick
OpenSSH launches. It does not replace OpenSSH; it keeps connection metadata in a
local SQLite database, keeps passwords/passphrases in an encrypted vault, and
starts the system `ssh` client with the right options.

## Features

- Bubble Tea TUI for daily interactive use.
- CLI commands for scripting and quick edits.
- Encrypted vault for SSH passwords and key passphrases.
- Password and key-passphrase auth through a PTY prompt handler, without putting
  secrets in command-line arguments.
- Key, SSH-agent, password, and key+passphrase auth modes.
- Groups, tags, command templates, search, and OpenSSH config generation.
- Import from `~/.ssh/config`.

## Install

```bash
git clone https://git.mirv.top/mirivlad/sshkeeper.git
cd sshkeeper
go build -o ~/.local/bin/sshkeeper .
```

Requirements: Go 1.25+, Linux x86_64, system OpenSSH.

## First Run

Run the TUI or any command. On the first run, `sshkeeper` creates its config,
database, and vault, then asks for a master password.

```bash
sshkeeper
```

You can also initialize explicitly:

```bash
sshkeeper init
```

## Common CLI Commands

```bash
# Add profiles with flags
sshkeeper add web --host 10.0.0.10 --user deploy --auth key
sshkeeper add prod --host 10.0.0.20 --user root --auth password
sshkeeper add bastion --host bastion.example.org --user admin --auth key_passphrase --identity-file ~/.ssh/id_rsa

# Or use the interactive CLI prompt
sshkeeper add

# Inspect profiles
sshkeeper list
sshkeeper show web
sshkeeper search prod

# Connect and test
sshkeeper connect web
sshkeeper c web
sshkeeper test web
sshkeeper run web "uptime"

# Groups and templates
sshkeeper group list
sshkeeper template list web

# OpenSSH config
sshkeeper ssh-config generate
sshkeeper ssh-config install-include
```

Commands that only read profile metadata, such as `list`, `show`, `search`,
`config path`, `group list`, and `export`, do not require the master password.
Commands that need secrets ask for the master password in that process. Adding
`key` or `agent` profiles does not require unlocking the vault; adding
`password` or `key_passphrase` profiles asks for the master password before
storing the secret.

## TUI

Running `sshkeeper` without arguments opens the TUI.

| Key | Action |
| --- | --- |
| Enter | Connect to selected server |
| Ctrl+A | Add server |
| Ctrl+E | Edit server |
| Ctrl+D | Delete server |
| Ctrl+T | Test connection |
| Ctrl+F | Search |
| Ctrl+Q / Ctrl+C | Quit |

In add/edit forms:

| Key | Action |
| --- | --- |
| Tab / Down | Next field |
| Shift+Tab / Up | Previous field |
| `/` on Auth Method or Group | Pick from list |
| Enter | Move to action / activate |
| Esc | Back |

## Vault

The vault stores SSH passwords and key passphrases encrypted on disk.

- Cipher: XChaCha20-Poly1305.
- KDF: Argon2id, currently 64 MiB memory, 3 iterations.
- Existing legacy vault files remain readable.
- Unlock state is process-local. `sshkeeper vault unlock` verifies the master
  password, but it does not keep future shell commands unlocked.

Useful commands:

```bash
sshkeeper vault status
sshkeeper vault unlock
sshkeeper vault list
sshkeeper vault delete <alias> [ssh_password|key_passphrase]
sshkeeper vault change-password
```

`vault list`, `vault delete`, and `vault change-password` ask for the master
password themselves because they need to decrypt the vault in the current
process.

## Data Locations

`sshkeeper` uses XDG-style app directories:

| Data | Default path |
| --- | --- |
| Config | `~/.config/sshkeeper/config.toml` |
| Database | `~/.local/share/sshkeeper/sshkeeper.db` |
| Vault | `~/.local/share/sshkeeper/vault.bin` |
| Generated OpenSSH config | `~/.ssh/config.d/sshkeeper.conf` |

If `XDG_CONFIG_HOME` or `XDG_DATA_HOME` are set, sshkeeper stores data under
`$XDG_CONFIG_HOME/sshkeeper` and `$XDG_DATA_HOME/sshkeeper`.

## Build And Test

```bash
go test ./...
go build -o bin/sshkeeper .
```

`bin/` is ignored by git.

## Project Layout

```text
sshkeeper/
├── cmd/                 # Cobra CLI commands and TUI launcher
├── internal/config/     # XDG paths and config loading
├── internal/db/         # SQLite migrations and CRUD
├── internal/model/      # Domain models
├── internal/ssh/        # OpenSSH command building, PTY prompt handling
├── internal/tui/        # Bubble Tea UI
├── internal/vault/      # Encrypted vault
└── main.go
```

## License

MIT
