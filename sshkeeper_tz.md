# ТЗ для ИИ-кодера: sshkeeper — консольный менеджер SSH-подключений для Linux

## 1. Назначение проекта

Нужно разработать консольное приложение для Linux, которое работает как менеджер SSH-серверов и учётных данных.

Приложение **не должно реализовывать собственный SSH-клиент с нуля**. Оно должно использовать системный OpenSSH (`/usr/bin/ssh`) как реальный транспорт подключения, а само приложение должно быть удобным слоем управления над:

- списком серверов;
- пользователями;
- портами;
- SSH-ключами;
- SSH-паролями;
- группами;
- тегами;
- заметками;
- bastion/proxyjump;
- локальными настройками подключения;
- encrypted vault для секретов.

Рабочее название приложения: **sshkeeper**.

Главная идея:

> `sshkeeper` не заменяет OpenSSH.  
> `sshkeeper` управляет профилями подключений, секретами и удобным запуском SSH-сессий.

---

## 2. Целевая платформа

Основная целевая платформа:

- Linux x86_64;
- Arch Linux;
- Debian/Ubuntu;
- Fedora-compatible дистрибутивы.

На первом этапе Windows и macOS не обязательны.

---

## 3. Основной стек

Использовать:

- Go;
- Cobra для CLI;
- Bubble Tea для TUI;
- Bubbles для TUI-компонентов;
- SQLite для локальной базы профилей;
- `modernc.org/sqlite` как SQLite-драйвер без CGO;
- `golang.org/x/crypto/argon2` для Argon2id KDF;
- `golang.org/x/crypto/chacha20poly1305` для XChaCha20-Poly1305;
- `github.com/creack/pty` для запуска OpenSSH через PTY;
- системный `/usr/bin/ssh`.

Не использовать:

- Electron;
- GUI;
- внешние password managers как обязательную зависимость;
- GNOME Keyring/KWallet/libsecret как обязательную зависимость;
- хранение паролей в plaintext;
- передачу пароля через аргументы командной строки;
- передачу пароля через environment variables.

---

## 4. Основные пользовательские сценарии

### 4.1. Добавить сервер

Пользователь запускает:

```bash
sshkeeper add
```

Приложение открывает интерактивную форму в терминале.

Поля:

- Alias;
- Display name;
- Host;
- Port;
- User;
- Auth method:
  - `password`;
  - `key`;
  - `key+passphrase`;
  - `agent`;
- Identity file, если используется ключ;
- Password, если используется пароль;
- Key passphrase, если используется ключ с passphrase;
- Group;
- Tags;
- Notes;
- ProxyJump, опционально;
- Local forwards, опционально;
- Remote forwards, опционально.

В конце формы должны быть две основные кнопки:

```text
[Test] [Save]
```

---

### 4.2. Кнопка Test

Кнопка `Test` проверяет текущие введённые данные.

Важно:

- `Test` **не сохраняет** профиль.
- `Test` **не сохраняет** новые секреты в постоянный vault.
- `Test` использует введённые данные только из текущей формы.
- Если подключение успешно — показать `Connection OK`.
- Если подключение неуспешно — показать ошибку.
- После теста пользователь остаётся в форме.
- Все введённые значения должны сохраниться в форме.
- Пользователь сам решает, нажимать ли потом `Save`.

Пример успешного теста:

```text
Connection OK.
```

Пример ошибки пароля:

```text
Connection failed:

Permission denied, please try again.
```

Пример ошибки ключа:

```text
Connection failed:

Identity file ~/.ssh/prod_ed25519 not found.
```

Пример сетевой ошибки:

```text
Connection failed:

connect to host 10.0.0.11 port 22: No route to host.
```

---

### 4.3. Кнопка Save

Кнопка `Save` сохраняет профиль как есть.

Важно:

- `Save` **не обязан** выполнять тест подключения.
- `Save` **не должен** блокировать сохранение, если сервер недоступен.
- `Save` **не должен** показывать раздражающие предупреждения про опасность SSH-паролей.
- Если пользователь выбрал `password`, пароль сохраняется в собственный encrypted vault.
- Если пользователь выбрал `key+passphrase`, passphrase сохраняется в encrypted vault.
- Обычные данные профиля сохраняются в SQLite.
- Секреты не должны попадать в SQLite в открытом виде.

После сохранения показать:

```text
Saved.
```

---

## 5. Пароли как штатный режим

Парольная авторизация должна быть полноценным штатным режимом.

Не нужно делать предупреждения вида:

```text
WARNING! Password auth is insecure!
```

Такие предупреждения не нужны.

Логика:

- `auth=password` — нормальный режим.
- Пароль хранится в собственном encrypted vault.
- Пароль не хранится в YAML/TOML/SQLite открытым текстом.
- Пароль не передаётся в argv.
- Пароль не передаётся через env.
- Пароль не пишется в логи.
- Пароль не показывается в интерфейсе после ввода.
- В списке серверов можно показывать тип авторизации: `password`, `key`, `agent`, `key+passphrase`.

---

## 6. Структура хранения данных

Использовать XDG-совместимые пути.

База данных:

```text
~/.local/share/sshkeeper/sshkeeper.db
```

Vault:

```text
~/.local/share/sshkeeper/vault.bin
```

Конфиг приложения:

```text
~/.config/sshkeeper/config.toml
```

Сгенерированный OpenSSH config:

```text
~/.ssh/config.d/sshkeeper.conf
```

Пользовательский `~/.ssh/config` может содержать:

```sshconfig
Include ~/.ssh/config.d/*.conf
```

Если такой строки нет, приложение должно уметь добавить её отдельной командой:

```bash
sshkeeper ssh-config install-include
```

Делать это автоматически без явного действия пользователя не нужно.

---

## 7. SQLite-схема MVP

Минимальные таблицы:

---

### 7.1. `servers`

Поля:

- `id`;
- `alias`;
- `display_name`;
- `host`;
- `port`;
- `user`;
- `auth_method`;
- `identity_file`;
- `proxy_jump`;
- `group_name`;
- `notes`;
- `created_at`;
- `updated_at`;
- `last_connected_at`;
- `last_test_at`;
- `last_test_status`;
- `last_test_error`.

`auth_method` значения:

- `password`;
- `key`;
- `key_passphrase`;
- `agent`.

`last_test_status` значения:

- `unknown`;
- `ok`;
- `failed`.

---

### 7.2. `tags`

Поля:

- `id`;
- `name`.

---

### 7.3. `server_tags`

Поля:

- `server_id`;
- `tag_id`.

---

### 7.4. `forwards`

Поля:

- `id`;
- `server_id`;
- `type`;
- `local_addr`;
- `local_port`;
- `remote_addr`;
- `remote_port`.

`type` значения:

- `local`;
- `remote`;
- `dynamic`.

---

### 7.5. `command_templates`

Поля:

- `id`;
- `server_id`;
- `name`;
- `command`.

Примеры command templates:

- `logs` → `journalctl -xe`;
- `docker` → `docker ps`;
- `nginx-test` → `nginx -t`.

---

## 8. Vault

Нужен собственный encrypted vault.

---

### 8.1. Master password

При первом запуске:

```bash
sshkeeper init
```

Приложение спрашивает:

```text
Create master password:
Repeat master password:
```

Master password не хранится.

Из master password через Argon2id выводится master key.

---

### 8.2. KDF

Использовать Argon2id.

Начальные параметры:

```text
memory: 64 MiB
iterations: 3
parallelism: 1
salt: random 16 или 32 bytes
key length: 32 bytes
```

Параметры KDF должны храниться в metadata vault, чтобы в будущем можно было менять настройки.

---

### 8.3. Шифрование

Использовать XChaCha20-Poly1305.

Каждая запись vault должна иметь отдельный случайный nonce.

Формат vault можно сделать JSON или бинарный. Для MVP допустим JSON с base64-полями.

Пример структуры:

```json
{
  "version": 1,
  "kdf": {
    "name": "argon2id",
    "memory_kib": 65536,
    "iterations": 3,
    "parallelism": 1,
    "salt": "base64..."
  },
  "records": [
    {
      "id": "server:old-router:ssh-password",
      "type": "ssh_password",
      "nonce": "base64...",
      "ciphertext": "base64..."
    }
  ]
}
```

---

### 8.4. Типы секретов

Поддержать типы:

- `ssh_password`;
- `key_passphrase`;
- `sudo_password`;
- `custom_secret`.

Для MVP обязательны:

- `ssh_password`;
- `key_passphrase`.

---

### 8.5. Secret references

В SQLite хранить только ссылки на секреты.

Примеры:

```text
server:old-router:ssh-password
server:prod-web-1:key-passphrase
server:prod-web-1:sudo-password
```

Секреты должны лежать только в vault.

---

## 9. Подключение к серверу

Команда:

```bash
sshkeeper connect <alias>
```

Короткий алиас:

```bash
sshkeeper c <alias>
```

Логика:

1. Найти профиль сервера в SQLite.
2. Если нужен vault — запросить master password, если vault ещё не разблокирован.
3. Сформировать команду `ssh`.
4. Запустить системный `/usr/bin/ssh`.
5. Если используется пароль — запустить SSH через PTY-wrapper.
6. Дождаться password prompt.
7. Отправить пароль в PTY.
8. Передать управление пользователю.
9. После завершения сессии обновить `last_connected_at`.

---

## 10. PTY-wrapper для паролей

Для `auth=password` нельзя передавать пароль через аргументы командной строки.

Нужен PTY-wrapper.

Примерная логика:

```text
start ssh through PTY
read PTY output
detect password prompt
write password + "\n"
after login, bridge stdin/stdout/stderr between user terminal and PTY
handle terminal resize
restore terminal state on exit
```

Prompt detection должен учитывать варианты:

```text
password:
Password:
user@host's password:
Enter password:
```

Для MVP достаточно английских вариантов.

Позже можно добавить расширяемые regex-шаблоны в config.

---

## 11. Проверка подключения

Команда:

```bash
sshkeeper test <alias>
```

В форме сервера кнопка:

```text
[Test]
```

Проверка должна выполнять короткую безопасную команду:

```bash
echo SSHKEEPER_OK
```

Ожидаемый вывод:

```text
SSHKEEPER_OK
```

Если команда выполнилась и вывод получен — тест успешен.

Для нестандартных систем можно добавить fallback:

```bash
exit
```

Но для MVP достаточно `echo SSHKEEPER_OK`.

Важно:

- Тест из формы не сохраняет профиль.
- Тест существующего профиля обновляет:
  - `last_test_at`;
  - `last_test_status`;
  - `last_test_error`.
- Тест должен иметь timeout.
- Начальный timeout: 10 секунд.
- Timeout должен настраиваться в config.

---

## 12. CLI-команды MVP

Обязательные команды:

```bash
sshkeeper init
sshkeeper add
sshkeeper list
sshkeeper show <alias>
sshkeeper edit <alias>
sshkeeper delete <alias>
sshkeeper connect <alias>
sshkeeper c <alias>
sshkeeper test <alias>
sshkeeper search <query>
sshkeeper vault lock
sshkeeper vault unlock
sshkeeper vault status
sshkeeper vault change-password
sshkeeper config path
sshkeeper ssh-config generate
sshkeeper ssh-config install-include
```

Дополнительные команды, если останется время:

```bash
sshkeeper import ~/.ssh/config
sshkeeper export
sshkeeper run <alias> <command>
sshkeeper run-template <alias> <template>
sshkeeper group list
sshkeeper group test <group>
```

---

## 13. TUI MVP

При запуске без аргументов:

```bash
sshkeeper
```

Открывается TUI.

Главный экран:

```text
┌─ sshkeeper ─────────────────────────────────────────────┐
│ Search: _                                                │
├──────────────────────────────────────────────────────────┤
│ [?] home-nextcloud   admin@192.168.1.15:22    key        │
│ [✓] prod-web-1       root@10.0.0.11:22        key        │
│ [!] old-router       admin@192.168.1.1:22     password   │
└──────────────────────────────────────────────────────────┘

Enter connect | a add | e edit | d delete | t test | / search | q quit
```

Статусы:

```text
[?] never tested
[✓] last test OK
[!] last test failed
```

Клавиши:

- `Enter` — connect;
- `a` — add;
- `e` — edit;
- `d` — delete;
- `t` — test;
- `/` — search;
- `q` — quit;
- `Esc` — назад/отмена.

Форма добавления/редактирования должна иметь две основные кнопки:

```text
[Test] [Save]
```

`Test` проверяет, но не сохраняет.  
`Save` сохраняет без обязательного теста.

---

## 14. Генерация OpenSSH config

Команда:

```bash
sshkeeper ssh-config generate
```

Должна создавать файл:

```text
~/.ssh/config.d/sshkeeper.conf
```

Для серверов с ключами можно генерировать:

```sshconfig
Host prod-web-1
    HostName 10.0.0.11
    User root
    Port 22
    IdentityFile ~/.ssh/prod_ed25519
    ProxyJump bastion
```

Для password-auth профилей тоже можно генерировать базовый host без пароля:

```sshconfig
Host old-router
    HostName 192.168.1.1
    User admin
    Port 22
```

Пароли в ssh config не писать никогда.

---

## 15. Логи

Логи должны быть аккуратными.

Не логировать:

- SSH-пароли;
- key passphrase;
- master password;
- decrypted vault content.

Можно логировать:

- alias;
- host;
- port;
- user;
- auth method;
- ошибку подключения без секретов.

Логи MVP можно писать только в stderr/debug mode.

---

## 16. Безопасность файлов

При создании файлов выставлять права:

```text
~/.local/share/sshkeeper/sshkeeper.db    0600
~/.local/share/sshkeeper/vault.bin       0600
~/.config/sshkeeper/config.toml          0600
~/.ssh/config.d/sshkeeper.conf           0600
```

Директории:

```text
~/.local/share/sshkeeper                 0700
~/.config/sshkeeper                      0700
~/.ssh/config.d                          0700 или существующие безопасные права
```

---

## 17. Конфиг приложения

Файл:

```text
~/.config/sshkeeper/config.toml
```

Пример:

```toml
[ssh]
binary = "/usr/bin/ssh"
connect_timeout_seconds = 10
test_command = "echo SSHKEEPER_OK"

[vault]
auto_lock_minutes = 15

[ui]
show_security_hints = false
```

---

## 18. Архитектура кода

Предлагаемая структура проекта:

```text
sshkeeper/
  go.mod
  go.sum
  main.go

  cmd/
    root.go
    init.go
    add.go
    list.go
    show.go
    edit.go
    delete.go
    connect.go
    test.go
    search.go
    vault.go
    ssh_config.go

  internal/
    app/
      app.go

    config/
      config.go
      paths.go

    db/
      db.go
      migrations.go
      servers.go
      tags.go
      forwards.go

    model/
      server.go
      secret.go
      forward.go
      tag.go

    vault/
      vault.go
      crypto.go
      format.go
      unlock.go

    ssh/
      command.go
      launcher.go
      pty.go
      test.go
      configgen.go

    tui/
      app.go
      list.go
      form.go
      styles.go

    prompt/
      input.go
      password.go

    util/
      fs.go
      time.go
      errors.go
```

---

## 19. Ошибки и UX

Ошибки должны быть понятными человеку.

Плохо:

```text
exit status 255
```

Хорошо:

```text
Connection failed:

Permission denied.

Server: old-router
Target: admin@192.168.1.1:22
Auth: password
```

При этом не надо добавлять длинные лекции и предупреждения.

---

## 20. Этапы разработки

### Этап 1. Каркас CLI

Сделать:

- Go module;
- Cobra root command;
- команды:
  - `init`;
  - `add`;
  - `list`;
  - `show`;
  - `delete`;
- XDG paths;
- создание директорий;
- базовый config.

Критерий готовности:

```bash
sshkeeper init
sshkeeper add
sshkeeper list
sshkeeper show test-server
```

Команды работают.

---

### Этап 2. SQLite

Сделать:

- SQLite подключение;
- migrations;
- таблицу `servers`;
- CRUD серверов;
- хранение tags/groups/notes можно пока упростить.

Критерий готовности:

- серверы сохраняются между запусками;
- можно добавить, посмотреть, удалить сервер.

---

### Этап 3. Vault

Сделать:

- `vault.bin`;
- master password;
- Argon2id key derivation;
- XChaCha20-Poly1305 encryption;
- команды:
  - `vault unlock`;
  - `vault lock`;
  - `vault status`;
  - `vault change-password`;
- сохранение `ssh_password`;
- сохранение `key_passphrase`.

Критерий готовности:

- пароль можно сохранить;
- пароль не виден в SQLite;
- после перезапуска пароль можно достать только через master password.

---

### Этап 4. Connect через OpenSSH

Сделать:

- `sshkeeper connect <alias>`;
- `sshkeeper c <alias>`;
- запуск `/usr/bin/ssh`;
- key auth;
- agent auth;
- password auth через PTY-wrapper.

Критерий готовности:

- можно подключиться к серверу по ключу;
- можно подключиться к серверу по паролю;
- пароль не передаётся через argv/env.

---

### Этап 5. Test

Сделать:

- `sshkeeper test <alias>`;
- test command: `echo SSHKEEPER_OK`;
- timeout;
- обновление:
  - `last_test_at`;
  - `last_test_status`;
  - `last_test_error`.

Критерий готовности:

- успешное подключение отмечается `[✓]`;
- неуспешное — `[!]`;
- ошибка сохраняется и показывается.

---

### Этап 6. TUI

Сделать:

- запуск TUI при `sshkeeper`;
- список серверов;
- поиск;
- connect;
- add form;
- edit form;
- delete;
- test;
- кнопки `[Test] [Save]`.

Критерий готовности:

- приложением можно пользоваться без знания CLI-команд;
- добавление/редактирование сервера возможно из TUI;
- `Test` не сохраняет;
- `Save` сохраняет без обязательного теста.

---

### Этап 7. OpenSSH config generation

Сделать:

- `sshkeeper ssh-config generate`;
- генерация `~/.ssh/config.d/sshkeeper.conf`;
- `sshkeeper ssh-config install-include`;
- не писать пароли в ssh config.

Критерий готовности:

- после генерации можно выполнить:

```bash
ssh alias
```

для key/agent/password профилей, где password всё равно будет спрашиваться самим ssh.

---

### Этап 8. Полировка

Сделать:

- импорт из `~/.ssh/config`;
- export/backup;
- groups;
- command templates;
- run command;
- group test;
- fuzzy search;
- нормальные help-сообщения.

---

## 21. Минимальный acceptance checklist

Приложение считается MVP-готовым, если:

- `sshkeeper init` создаёт конфиг, БД и vault.
- `sshkeeper add` добавляет сервер.
- `sshkeeper list` показывает серверы.
- `sshkeeper show <alias>` показывает профиль без секретов.
- `sshkeeper edit <alias>` редактирует профиль.
- `sshkeeper delete <alias>` удаляет профиль.
- `sshkeeper c <alias>` подключается через OpenSSH.
- password-auth работает через PTY.
- key-auth работает.
- key+passphrase работает.
- пароль не хранится в plaintext.
- пароль не передаётся через argv/env.
- `sshkeeper test <alias>` проверяет подключение.
- TUI запускается командой `sshkeeper`.
- В TUI есть список серверов.
- В TUI есть форма add/edit.
- В форме есть кнопки `Test` и `Save`.
- `Test` не сохраняет.
- `Save` сохраняет без обязательного теста.
- `sshkeeper ssh-config generate` создаёт OpenSSH config без паролей.

---

# Инструкция по настройке среды разработки, сборке и запуску

## 1. Установить зависимости ОС

### Arch Linux

```bash
sudo pacman -Syu
sudo pacman -S go git openssh make
```

### Debian/Ubuntu

```bash
sudo apt update
sudo apt install -y golang-go git openssh-client make
```

Если нужна свежая версия Go, лучше установить Go с официального сайта, а не из репозитория дистрибутива.

Проверить:

```bash
go version
git --version
ssh -V
```

---

## 2. Создать проект

```bash
mkdir -p ~/projects
cd ~/projects
mkdir sshkeeper
cd sshkeeper
go mod init github.com/mirivlad/sshkeeper
```

---

## 3. Установить зависимости Go

```bash
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get modernc.org/sqlite@latest
go get golang.org/x/crypto@latest
go get github.com/creack/pty@latest
```

---

## 4. Создать минимальный `main.go`

```go
package main

import "github.com/mirivlad/sshkeeper/cmd"

func main() {
	cmd.Execute()
}
```

---

## 5. Создать `Makefile`

```makefile
APP=sshkeeper

.PHONY: build run test clean install

build:
	go build -o bin/$(APP) .

run:
	go run .

test:
	go test ./...

clean:
	rm -rf bin

install:
	go build -o $(HOME)/.local/bin/$(APP) .
```

---

## 6. Собрать приложение

```bash
make build
```

Проверить:

```bash
./bin/sshkeeper --help
```

---

## 7. Запустить из исходников

```bash
go run . --help
go run . init
go run . list
```

---

## 8. Установить локально в PATH

Убедиться, что есть директория:

```bash
mkdir -p ~/.local/bin
```

Добавить в `~/.bashrc` или `~/.zshrc`, если ещё не добавлено:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Применить:

```bash
source ~/.bashrc
```

или:

```bash
source ~/.zshrc
```

Установить:

```bash
make install
```

Проверить:

```bash
sshkeeper --help
```

---

## 9. Первый запуск

```bash
sshkeeper init
```

Ожидаемый результат:

```text
Created config: ~/.config/sshkeeper/config.toml
Created database: ~/.local/share/sshkeeper/sshkeeper.db
Created vault: ~/.local/share/sshkeeper/vault.bin
```

---

## 10. Добавить тестовый сервер

Интерактивно:

```bash
sshkeeper add
```

Или позже можно реализовать неинтерактивный вариант:

```bash
sshkeeper add test-vps \
  --host 192.168.1.10 \
  --port 22 \
  --user root \
  --auth key \
  --identity-file ~/.ssh/id_ed25519
```

---

## 11. Проверить подключение

```bash
sshkeeper test test-vps
```

---

## 12. Подключиться

```bash
sshkeeper c test-vps
```

---

## 13. Запустить TUI

```bash
sshkeeper
```

---

## 14. Сгенерировать OpenSSH config

```bash
sshkeeper ssh-config generate
```

При необходимости добавить Include:

```bash
sshkeeper ssh-config install-include
```

После этого для части профилей можно будет подключаться напрямую:

```bash
ssh test-vps
```

---

## 15. Рекомендации к разработке

После каждого этапа запускать:

```bash
go fmt ./...
go vet ./...
go test ./...
make build
```

Если есть ошибки — исправить до перехода к следующему этапу.

Не переходить к TUI, пока CLI, SQLite, vault и connect не работают стабильно.

---

# Примечания по философии проекта

`sshkeeper` должен оставаться слоем управления, а не пытаться стать новым OpenSSH.

OpenSSH уже умеет:

- `Host`;
- `HostName`;
- `User`;
- `Port`;
- `IdentityFile`;
- `ProxyJump`;
- `LocalForward`;
- `RemoteForward`;
- `Include`.

Поэтому `sshkeeper` должен:

- использовать OpenSSH как транспорт;
- генерировать совместимый config;
- хранить секреты отдельно;
- давать удобный CLI/TUI;
- не спорить с пользователем;
- помогать, но не блокировать.

Главное UX-правило:

> `Test` проверяет.  
> `Save` сохраняет.  
> Пользователь сам решает, что ему нужно.
