# sshkeeper

`sshkeeper` is a Linux console manager for SSH profiles, secrets, and quick
OpenSSH launches. It does not replace OpenSSH; it keeps connection metadata in a
local SQLite database, keeps passwords/passphrases in an encrypted vault, and
starts the system `ssh` client with the right options.

## sshkeeper is not Ansible

sshkeeper does not configure servers, push files, or manage infrastructure.
It is an SSH connection manager: it remembers how to reach your servers
(bastions, jump chains, port forwards) and launches the system `ssh` client.
Think of it as a smart `~/.ssh/config` with a TUI, encrypted secrets, and
port forwarding management.

## Features

- Bubble Tea TUI for daily interactive use.
- CLI commands for scripting and quick edits.
- Encrypted vault for SSH passwords and key passphrases.
- Password and key-passphrase auth through a PTY prompt handler, without putting
  secrets in command-line arguments.
- Key, SSH-agent, password, and key+passphrase auth modes.
- **Routes / ProxyJump** — manage bastion hosts and jump chains with human-readable display.
- **Port forwarding** — local, remote, and dynamic (SOCKS) forwards with OpenSSH preview.
- **Tunnel mode** — `ssh -N` for forward-only sessions.
- Groups, tags, command templates, search, and OpenSSH config generation.
- Import from `~/.ssh/config`.

## Install

### Install from release

Download the latest Linux x86_64 release from:

https://github.com/mirivlad/sshkeeper/releases/latest

```bash
tar -xzf sshkeeper_v0.2.0_linux_amd64.tar.gz
chmod +x sshkeeper-linux-amd64
sudo install -m 0755 sshkeeper-linux-amd64 /usr/local/bin/sshkeeper
sshkeeper
```

### Build from source

```bash
git clone https://github.com/mirivlad/sshkeeper.git
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
sshkeeper template list
sshkeeper template add uptime "uptime"
sshkeeper run-template web uptime

# Tags and startup command
sshkeeper add web --host 10.0.0.10 --user deploy --auth key --tags prod,web --startup-command "tmux attach -t ops"
sshkeeper edit web --tags prod,web --startup-command "tmux attach -t ops"

# OpenSSH config
sshkeeper ssh-config generate
sshkeeper ssh-config install-include

## Routes, Tunnels, and Port Forwards

### Jump host (single bastion)

```bash
sshkeeper route set web --jumps bastion
sshkeeper route show web
# Route: bastion → web@10.0.0.10:22
# Mode: via
# ProxyJump: bastion
```

### Jump chain (multiple hops)

```bash
sshkeeper route set prod --jumps bastion,dmz-gw
sshkeeper route show prod
# Route: bastion → dmz-gw → prod@10.0.0.20:22
# Mode: chain
# ProxyJump: bastion,dmz-gw
```

### Local port forward

```bash
sshkeeper forward add web --type local --local-port 8080 --remote-addr internal.web --remote-port 80
sshkeeper forward list web
# [1] -L 0.0.0.0:8080:internal.web:80
```

### Dynamic SOCKS proxy

```bash
sshkeeper forward add bastion --type dynamic --local-port 1080
sshkeeper forward list bastion
# [1] -D 0.0.0.0:1080
```

### Forward-only tunnel (ssh -N)

```bash
sshkeeper tunnel web --forward-only
# Starting tunnel to web with 1 forward(s)...
# Tunnel mode (ssh -N). Press Ctrl+C to exit.
```

### Session with forwards

```bash
sshkeeper tunnel web
# Starts SSH session with all configured forwards active.
```

## Connect vs Tunnel

- **`sshkeeper connect <alias>`** (or `Enter` in TUI) — standard SSH session, no port forwards.
- **`sshkeeper forward`** (or `Ctrl+W` in TUI) — manage port forwards for a server.
- **`sshkeeper tunnel <alias>`** — start SSH session **with** all configured forwards active.
- **`sshkeeper tunnel <alias> --forward-only`** — start tunnel only (`ssh -N`), useful for background port forwarding.
- **TUI Action Menu** (`Ctrl+X`) — offers Connect, Start tunnel, and Tunnel mode for the selected server.

Commands that only read profile metadata, such as `list`, `show`, `search`,
`config path`, `group list`, and `export`, do not require the master password.
Commands that need secrets ask for the master password in that process. Adding
`key` or `agent` profiles does not require unlocking the vault; adding
`password` or `key_passphrase` profiles asks for the master password before
storing the secret.

## TUI

Running `sshkeeper` without arguments opens the TUI.

## Screenshots

### Main Window

![sshkeeper main window](docs/screenshots/screen_1.png)

### Edit Server

![sshkeeper edit server form](docs/screenshots/screen_2.png)

![sshkeeper group picker](docs/screenshots/screen_3.png)

### Template Manager

![sshkeeper template manager](docs/screenshots/screen_4.png)

### Route and Forwarding

![sshkeeper route screen](docs/screenshots/screen_5_route.png)

![sshkeeper port forwards](docs/screenshots/screen_6_forwards.png)

| Key | Action |
| --- | --- |
| Enter | Connect to selected server |
| Ctrl+R | Pick and run a command template on the selected servers |
| Insert | Select or unselect a server, then move to the next row |
| Ctrl+A | Add server |
| Ctrl+E | Edit server |
| Ctrl+D | Delete server |
| Ctrl+T | Test connection |
| Ctrl+F | Search |
| Ctrl+G | Manage tags |
| Ctrl+P | Manage global command templates |
| Ctrl+W | Manage port forwards for selected server |
| ? / F1 | Full help screen |
| Ctrl+X | Action menu (delete, test, tags, vault) |
| Ctrl+Q / Ctrl+C | Quit |

Templates are global entities and can run on any server. Foreground template
runs leave the TUI, show the SSH session in the terminal, and then return to the
TUI. Background runs execute the command and show per-server output in a result
screen.

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

## Security

`sshkeeper` stores SSH passwords and key passphrases in an encrypted local vault
and avoids passing secrets through command-line arguments. The project has not
had an independent security audit; review the implementation and threat model
before using it for high-risk environments.

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

MIT. See [LICENSE](LICENSE).
