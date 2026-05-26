# sshkeeper

Консольный менеджер SSH-подключений для Linux. Управляет профилями серверов, секретами и запускает SSH-сессии через системный OpenSSH.

**sshkeeper не заменяет OpenSSH.** Он управляет профилями подключений, секретами и удобным запуском SSH-сессий.

## Возможности

- **TUI-интерфейс** на Bubble Tea — интерактивный терминальный интерфейс
- **CLI-команды** для скриптов и быстрых операций
- **Encrypted vault** (Argon2id + XChaCha20-Poly1305) для хранения паролей
- **Парольная авторизация** через PTY-wrapper (без передачи пароля в argv)
- **Подключение по ключу**, SSH-agent, key+passphrase
- **Группы и теги** для организации серверов
- **Шаблоны команд** для частых задач
- **Генерация OpenSSH config** из профилей
- **Импорт из ~/.ssh/config**
- **Тестирование подключения** без сохранения

## Установка

### Из исходников

```bash
git clone https://git.mirv.top/mirivlad/sshkeeper.git
cd sshkeeper
go build -o ~/.local/bin/sshkeeper .
```

Требования: Go 1.25+, Linux x86_64

## Быстрый старт

```bash
# Первый запуск — создание vault и мастер-пароля
sshkeeper init

# Или сразу запустить TUI (vault создастся автоматически)
sshkeeper

# Добавить сервер
sshkeeper add myserver --host 10.0.0.1 --user admin --auth key

# Добавить сервер с паролем
sshkeeper add prod-web --host 10.0.0.5 --user deploy --auth password --password

# Показать список серверов
sshkeeper list

# Подключиться к серверу
sshkeeper connect myserver
sshkeeper c myserver

# Проверить подключение
sshkeeper test myserver

# Запустить команду на сервере
sshkeeper run myserver "uptime"

# Группы
sshkeeper group list

# Редактировать сервер
sshkeeper edit myserver --host 10.0.0.2

# Удалить сервер
sshkeeper delete myserver

# Импорт из ~/.ssh/config
sshkeeper import

# Сгенерировать OpenSSH config
sshkeeper ssh-config generate
sshkeeper ssh-config install-include
```

## TUI

Запуск без аргументов открывает интерактивный терминальный интерфейс:

```bash
sshkeeper
```

Клавиши:

| Клавиша | Действие |
|---------|----------|
| Enter | Подключиться к серверу |
| a | Добавить сервер |
| e | Редктировать сервер |
| d | Удалить сервер |
| t | Проверить подключение |
| / | Поиск |
| q | Выход |

В форме добавления/редактирования:

| Клавиша | Действие |
|---------|----------|
| Tab/↓ | Следующее поле |
| Shift+Tab/↑ | Предыдущее поле |
| Enter | Перейти к кнопке / активировать |
| Esc | Назад |

Кнопки **[Test]** и **[Save]**:
- **Test** — проверяет подключение без сохранения
- **Save** — сохраняет профиль (не требует тест)

## Хранение данных

XDG-совместимые Пути:

| Файл | Путь |
|------|------|
| База данных | `~/.local/share/sshkeeper/sshkeeper.db` |
| Vault | `~/.local/share/sshkeeper/vault.bin` |
| Конфиг | `~/.config/sshkeeper/config.toml` |
| SSH config | `~/.ssh/config.d/sshkeeper.conf` |

## Vault

Vault — зашифрованное хранилище для паролей и passphrase.

**Шифрование:** XChaCha20-Poly1305  
**KDF:** Argon2id (4 MiB, 2 iterations)

При первом запуске sshkeeper создаёт vault и запрашивает мастер-пароль. При последующих запусках — запрашивает мастер-пароль для разблокировки.

```bash
# Разблокировать vault вручную
sshkeeper vault unlock

# Заблокировать
sshkeeper vault lock

# Сменить мастер-пароль
sshkeeper vault change-password

# Статус
sshkeeper vault status
```

## CLI-команды

```
sshkeeper                     TUI (по умолчанию)
sshkeeper add [alias]         Добавить сервер
sshkeeper list                Список серверов
sshkeeper show <alias>        Детали сервера
sshkeeper edit <alias>        Редактировать
sshkeeper delete <alias>      Удалить
sshkeeper connect <alias>     Подключиться (c — алиас)
sshkeeper test <alias>        Проверить подключение
sshkeeper search <query>      Поиск
sshkeeper run <alias> <cmd>   Выполнить команду
sshkeeper import              Импорт из ~/.ssh/config
sshkeeper export              Экспорт
sshkeeper group list          Группы
sshkeeper vault [subcommand]  Управление vault
sshkeeper ssh-config generate Генерация SSH config
```

## Сборка

```bash
# Собрать
make build

# Установить в ~/.local/bin
make install

# Запуск без сборки
go run .

# Тесты (если есть)
go test ./...
```

## Структура проекта

```
sshkeeper/
├── main.go                      # Точка входа
├── Makefile                     # Сборка
├── cmd/                         # CLI-команды
│   ├── root.go                  # Root command, initApp
│   ├── tui.go                   # TUI launcher
│   ├── add.go, edit.go, ...     # Команды
│   └── vault.go                 # Vault management
├── internal/
│   ├── config/                  # Конфигурация, XDG paths
│   ├── db/                      # SQLite, migrations, CRUD
│   ├── model/                   # Модели данных
│   ├── vault/                   # Encrypted vault (Argon2id + XChaCha20-Poly1305)
│   ├── ssh/                     # SSH connect, test, PTY-wrapper, import, configgen
│   └── tui/                     # Bubble Tea TUI
└── go.mod
```

## Лицензия

MIT
