package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbletea"
	"github.com/mirivlad/sshkeeper/internal/i18n"
	"github.com/mirivlad/sshkeeper/internal/model"
)

// --- Help screen (?) ---

type helpScreenModel struct {
	list   list.Model
	width  int
	height int
}

func newHelpScreenModel(w, h int) *helpScreenModel {
	items := []list.Item{
		helpScreenItem{key: "Enter", action: i18n.T("Connect / Confirm", "Подключиться / подтвердить")},
		helpScreenItem{key: "Esc", action: i18n.T("Back / Cancel", "Назад / отмена")},
		helpScreenItem{key: "Tab/↓", action: i18n.T("Next field", "Следующее поле")},
		helpScreenItem{key: "Shift+Tab/↑", action: i18n.T("Previous field", "Предыдущее поле")},
		helpScreenItem{key: "/", action: i18n.T("Open dropdown picker", "Открыть список выбора")},
		helpScreenItem{key: "Ctrl+A", action: i18n.T("Add server", "Добавить сервер")},
		helpScreenItem{key: "Ctrl+E", action: i18n.T("Edit server", "Изменить сервер")},
		helpScreenItem{key: "Ctrl+F", action: i18n.T("Search", "Поиск")},
		helpScreenItem{key: "Ctrl+X", action: i18n.T("Server actions", "Действия с сервером")},
		helpScreenItem{key: "m", action: i18n.T("Manage groups / tags / templates / tunnels / vault / settings", "Группы / теги / шаблоны / туннели / хранилище / настройки")},
		helpScreenItem{key: "Ins", action: i18n.T("Select / deselect", "Выбрать / снять выбор")},
		helpScreenItem{key: "Ctrl+W", action: i18n.T("Manage port-forward rules", "Управлять правилами проброса портов")},
		helpScreenItem{key: "?", action: i18n.T("This quick help", "Краткая справка")},
		helpScreenItem{key: "Ctrl+H", action: i18n.T("Full documentation", "Полная справка")},
		helpScreenItem{key: "Ctrl+Q", action: i18n.T("Quit", "Выйти")},
	}

	l := list.New(items, helpScreenDelegate{}, w, h-4)
	l.Title = i18n.T("sshkeeper — Quick Help", "sshkeeper — Краткая справка")
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return &helpScreenModel{list: l, width: w, height: h}
}

type helpScreenItem struct {
	key     string
	action  string
	section string
}

func (i helpScreenItem) Title() string       { return i.key }
func (i helpScreenItem) Description() string { return i.action }
func (i helpScreenItem) FilterValue() string { return i.key + " " + i.action }

type helpScreenDelegate struct{}

func (d helpScreenDelegate) Height() int                               { return 2 }
func (d helpScreenDelegate) Spacing() int                              { return 0 }
func (d helpScreenDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d helpScreenDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(helpScreenItem)
	if !ok {
		return
	}
	style := normalStyle
	if index == m.Index() {
		style = selectedRowStyle
	}
	keyStr := fmt.Sprintf("%-12s", i.key)
	actionStr := i.action
	line := hotkeyStyle.Render(keyStr) + helpTextStyle.Render(actionStr)
	w.Write([]byte(style.Render("  " + line + "\n")))
}

func (m *helpScreenModel) Init() tea.Cmd {
	return nil
}

func (m *helpScreenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyEnter:
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *helpScreenModel) View() string {
	items := m.list.Items()
	body := func(width, height int) string {
		innerRows := max(1, height-2)
		start, end := visibleServerRange(len(items), m.list.Index(), innerRows)
		lines := make([]string, 0, innerRows)
		for index := start; index < end; index++ {
			item, ok := items[index].(helpScreenItem)
			if !ok {
				continue
			}
			marker := "  "
			if index == m.list.Index() {
				marker = "> "
			}
			lines = append(lines, marker+padCells(item.key, 12)+" "+item.action)
		}
		return renderPaddedPanel(width, height, lines)
	}
	return renderScreenShell(screenShell{
		breadcrumb: i18n.T("Quick Help", "Краткая справка"),
		status:     i18n.T("Keyboard reference", "Горячие клавиши"),
		width:      m.width,
		height:     m.height,
		body:       body,
		footer: []helpItem{
			{Key: "↑/↓", Action: i18n.T("move", "перемещение")},
			{Key: "Ctrl+H", Action: i18n.T("full help", "полная справка")},
			{Key: "Esc", Action: i18n.T("back", "назад")},
		},
	})
}

// --- Full help (Ctrl+H) ---

type fullHelpModel struct {
	width  int
	height int
	offset int
}

func newFullHelpModel(w, h int) *fullHelpModel {
	return &fullHelpModel{width: w, height: h}
}

func (m *fullHelpModel) Init() tea.Cmd { return nil }

func (m *fullHelpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyEnter:
			return m, nil
		case tea.KeyRunes:
			switch msg.String() {
			case "q", "Q":
				return m, nil
			case "j", "J":
				m.offset++
			case "k", "K":
				if m.offset > 0 {
					m.offset--
				}
			}
		case tea.KeyDown:
			m.offset++
		case tea.KeyUp:
			if m.offset > 0 {
				m.offset--
			}
		case tea.KeyHome:
			m.offset = 0
		case tea.KeyEnd:
			m.offset = 100
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

func (m *fullHelpModel) View() string {
	sections := []struct {
		title string
		rows  [][2]string
	}{
		{i18n.T("What is sshkeeper", "Что такое sshkeeper"), [][2]string{
			{"", i18n.T("sshkeeper is a console SSH connection manager.", "sshkeeper — консольный менеджер SSH-подключений.")},
			{"", i18n.T("Linux and macOS are primary release targets; Windows is experimental.", "Основные платформы — Linux и macOS; поддержка Windows экспериментальная.")},
			{"", i18n.T("It stores server profiles, secrets, and launches the system ssh client.", "Он хранит профили серверов и секреты, запускает системный SSH-клиент.")},
			{"", ""},
		}},
		{i18n.T("Navigation", "Навигация"), [][2]string{
			{"↑/↓", i18n.T("Move through list", "Перемещение по списку")},
			{"Tab/↓", i18n.T("Next field", "Следующее поле")},
			{"Shift+Tab/↑", i18n.T("Previous field", "Предыдущее поле")},
			{"/", i18n.T("Open dropdown picker", "Открыть список выбора")},
		}},
		{i18n.T("Global actions", "Общие действия"), [][2]string{
			{"Enter", i18n.T("Select / Confirm / Open", "Выбрать / подтвердить / открыть")},
			{"Esc", i18n.T("Back / Cancel / Close", "Назад / отмена / закрыть")},
			{"?", i18n.T("Quick help (hotkeys)", "Краткая справка")},
			{"Ctrl+H", i18n.T("Full documentation", "Полная справка")},
			{"Ctrl+Q", i18n.T("Quit", "Выйти")},
		}},
		{i18n.T("Server list", "Список серверов"), [][2]string{
			{"Enter", i18n.T("Connect to server", "Подключиться к серверу")},
			{"Ctrl+A", i18n.T("Add server", "Добавить сервер")},
			{"Ctrl+E", i18n.T("Edit server", "Изменить сервер")},
			{"Ctrl+F", i18n.T("Search", "Поиск")},
			{"Ctrl+W", i18n.T("Manage port-forward rules", "Управлять правилами проброса портов")},
			{"Ctrl+X", i18n.T("Server actions", "Действия с сервером")},
			{"m", i18n.T("Manage global entities", "Управление общими разделами")},
			{"Ins", i18n.T("Select / deselect", "Выбрать / снять выбор")},
		}},
		{i18n.T("Server actions (Ctrl+X)", "Действия с сервером (Ctrl+X)"), [][2]string{
			{i18n.T("Connect", "Подключиться"), i18n.T("Standard SSH session", "Обычная SSH-сессия")},
			{i18n.T("Port-forward rules", "Правила проброса портов"), i18n.T("Add / edit / enable / delete saved rules", "Добавить / изменить / включить / удалить сохранённые правила")},
			{i18n.T("Connect with forwards", "Подключиться с пробросом"), i18n.T("SSH session + enabled forwarding rules", "SSH-сессия и включённые правила проброса")},
			{i18n.T("Start tunnel process (no shell)", "Запустить туннель без оболочки"), i18n.T("Run enabled forwards in foreground", "Запустить включённые правила в активном процессе")},
			{i18n.T("Start background tunnel process", "Запустить фоновый туннель"), i18n.T("Run enabled forwards in background", "Запустить включённые правила в фоне")},
			{i18n.T("SSH route", "SSH-маршрут"), i18n.T("Configure direct or bastion route", "Настроить прямой маршрут или через промежуточные узлы")},
			{i18n.T("Test connection", "Проверить соединение"), i18n.T("Check if server is reachable", "Проверить доступность сервера")},
			{i18n.T("Edit", "Изменить"), i18n.T("Edit server profile", "Изменить профиль сервера")},
			{i18n.T("Delete", "Удалить"), i18n.T("Remove server profile", "Удалить профиль сервера")},
		}},
		{i18n.T("Manage (m)", "Управление (m)"), [][2]string{
			{i18n.T("Groups", "Группы"), i18n.T("Create / rename / remove groups", "Создать / переименовать / удалить группы")},
			{i18n.T("Tags", "Теги"), i18n.T("Manage and apply tags", "Управлять тегами и назначать их")},
			{i18n.T("Command templates", "Шаблоны команд"), i18n.T("Manage reusable commands", "Управлять повторно используемыми командами")},
			{i18n.T("Running tunnels", "Работающие туннели"), i18n.T("View and stop tracked tunnel processes", "Просматривать и останавливать процессы туннелей")},
			{i18n.T("Settings", "Настройки"), i18n.T("Choose interface language", "Выбрать язык интерфейса")},
			{i18n.T("Import / Export", "Импорт / экспорт"), i18n.T("Move server profile data", "Переносить данные профилей серверов")},
			{i18n.T("Vault", "Хранилище"), i18n.T("Lock or change master password", "Заблокировать или сменить мастер-пароль")},
		}},
		{i18n.T("Routes / ProxyJump", "Маршруты / ProxyJump"), [][2]string{
			{"", i18n.T("Routes define how to reach a server through jump hosts.", "Маршрут определяет путь к серверу через промежуточные узлы.")},
			{"● direct", i18n.T("No jump host", "Без промежуточного узла")},
			{"→ via", i18n.T("One bastion", "Один промежуточный узел")},
			{"⇒ chain", i18n.T("Multiple bastions", "Несколько промежуточных узлов")},
			{"", ""},
			{"CLI:", "sshkeeper route set <alias> --jumps bastion"},
		}},
		{i18n.T("Port forwarding", "Проброс портов"), [][2]string{
			{"", i18n.T("A forward is a saved rule — just configuration.", "Проброс — сохранённое правило, а не работающий процесс.")},
			{i18n.T("Local", "Локальный"), i18n.T("Local port → remote service", "Локальный порт → удалённый сервис")},
			{i18n.T("Remote", "Удалённый"), i18n.T("Remote port → local service", "Удалённый порт → локальный сервис")},
			{"SOCKS", i18n.T("Dynamic SOCKS proxy through SSH", "Динамический SOCKS-прокси через SSH")},
			{"", ""},
			{"Ctrl+A", i18n.T("Add forward rule", "Добавить правило проброса")},
			{"Enter/Ctrl+E", i18n.T("Edit forward rule", "Изменить правило проброса")},
			{"Space", i18n.T("Enable / disable rule", "Включить / выключить правило")},
			{"Ctrl+B", i18n.T("Start background tunnel for this server", "Запустить фоновый туннель для сервера")},
			{"Ctrl+X", i18n.T("Choose a foreground or background mode", "Выбрать режим запуска туннеля")},
			{"Ctrl+D", i18n.T("Delete rule (with confirmation)", "Удалить правило (с подтверждением)")},
		}},
		{i18n.T("Tunnels", "Туннели"), [][2]string{
			{"", i18n.T("A tunnel is a running SSH process that activates forwards.", "Туннель — работающий процесс SSH, который активирует правила проброса.")},
			{"", i18n.T("At least one enabled forward is required.", "Нужно хотя бы одно включённое правило.")},
			{"", i18n.T("Background mode needs key or agent auth.", "Для фонового режима нужен ключ или SSH-агент.")},
			{"", i18n.T("Foreground mode stops with Ctrl+C.", "Активный процесс останавливается клавишами Ctrl+C.")},
			{"", ""},
			{"CLI:", "sshkeeper tunnel <alias>"},
			{"CLI:", "sshkeeper tunnel <alias> --forward-only"},
			{"CLI:", "sshkeeper tunnel <alias> --background"},
		}},
	}

	var lines []string
	for _, sec := range sections {
		lines = append(lines, sectionStyle.Copy().MarginTop(0).Render(sec.title))
		for _, row := range sec.rows {
			if row[0] == "" {
				lines = append(lines, "  "+row[1])
			} else {
				lines = append(lines, fmt.Sprintf("  %-16s %s", row[0], row[1]))
			}
		}
		lines = append(lines, "")
	}

	body := func(width, height int) string {
		capacity := max(1, height-2)
		start := min(m.offset, max(0, len(lines)-capacity))
		end := min(len(lines), start+capacity)
		return renderPaddedPanel(width, height, lines[start:end])
	}
	return renderScreenShell(screenShell{
		breadcrumb: i18n.T("Full Help", "Полная справка"),
		status:     i18n.Tf("line %d/%d", "строка %d/%d", min(m.offset+1, len(lines)), len(lines)),
		width:      m.width,
		height:     m.height,
		body:       body,
		footer: []helpItem{
			{Key: "↑/↓", Action: i18n.T("scroll", "прокрутка")},
			{Key: "Ctrl+H", Action: i18n.T("full help", "полная справка")},
			{Key: "Esc/Enter", Action: i18n.T("close", "закрыть")},
		},
	})
}

// --- Action menu ---

type actionMenuItem struct {
	label       string
	action      string
	description string
}

func (i actionMenuItem) Title() string       { return i.label }
func (i actionMenuItem) Description() string { return "" }
func (i actionMenuItem) FilterValue() string { return i.label }

type actionMenuModel struct {
	list            list.Model
	title           string
	width           int
	height          int
	serverID        int64
	serverAlias     string
	authMethod      model.AuthMethod
	enabledForwards int
	loadingForwards bool
	loadErr         error
	errorText       string
}

func (m *actionMenuModel) setServer(server *model.Server, count int, loading bool) {
	m.serverID = server.ID
	m.serverAlias = server.Alias
	m.authMethod = server.AuthMethod
	m.enabledForwards = count
	m.loadingForwards = loading
}

func (m *actionMenuModel) unavailableReason(action string) string {
	if m.serverAlias == "" {
		return ""
	}
	if action != "tunnel" && action != "tunnel_n" && action != "tunnel_bg" {
		return ""
	}
	if m.loadingForwards {
		return i18n.T("Checking enabled port forwards...", "Проверка включённых правил проброса...")
	}
	if m.loadErr != nil {
		return i18n.Tf("Cannot load port forwards: %v", "Не удалось загрузить правила проброса: %v", m.loadErr)
	}
	if m.enabledForwards == 0 {
		return i18n.T("No enabled port forwards. Add or enable a rule first.", "Нет включённых правил проброса. Добавьте или включите правило.")
	}
	if action == "tunnel_bg" && (m.authMethod == model.AuthPassword || m.authMethod == model.AuthKeyPassphrase) {
		return i18n.T("Background mode needs key or agent authentication; use a foreground mode.", "Для фонового режима нужен ключ или SSH-агент; используйте активный режим.")
	}
	return ""
}

func newActionMenuModel(w, h int, availability ...bool) *actionMenuModel {
	sessionsAvailable := len(availability) > 0 && availability[0]
	items := []list.Item{
		actionMenuItem{label: i18n.T("Connect", "Подключиться"), action: "connect", description: i18n.T("Open an interactive SSH session.", "Открыть интерактивную SSH-сессию.")},
	}
	if sessionsAvailable {
		items = append(items, actionMenuItem{label: i18n.T("Open in session", "Открыть в сессии"), action: "session_open", description: i18n.T("Open this server in a persistent tmux-backed SSH tab.", "Открыть сервер в постоянной SSH-вкладке tmux.")})
	}
	items = append(items,
		actionMenuItem{label: i18n.T("Port-forward rules", "Правила проброса портов"), action: "forwards", description: i18n.T("Add, edit, enable, or remove saved forwarding rules for this server; no process starts.", "Добавить, изменить, включить или удалить сохранённые правила; процесс не запускается.")},
		actionMenuItem{label: i18n.T("Connect with forwards", "Подключиться с пробросом"), action: "tunnel", description: i18n.T("Open an SSH session and activate enabled forwarding rules.", "Открыть SSH-сессию и активировать включённые правила проброса.")},
		actionMenuItem{label: i18n.T("Start tunnel process (no shell)", "Запустить туннель без оболочки"), action: "tunnel_n", description: i18n.T("Run enabled forwarding rules in the foreground without a shell.", "Запустить включённые правила в активном процессе без оболочки.")},
		actionMenuItem{label: i18n.T("Start background tunnel process", "Запустить фоновый туннель"), action: "tunnel_bg", description: i18n.T("Run enabled forwarding rules as a background SSH process.", "Запустить включённые правила в фоновом процессе SSH.")},
		actionMenuItem{label: i18n.T("SSH route", "SSH-маршрут"), action: "route", description: i18n.T("Configure the direct or bastion route used to reach this server; no tunnel starts.", "Настроить прямой маршрут или путь через промежуточные узлы; туннель не запускается.")},
		actionMenuItem{label: i18n.T("Test connection", "Проверить соединение"), action: "test", description: i18n.T("Check SSH reachability for this profile.", "Проверить доступность профиля по SSH.")},
		actionMenuItem{label: i18n.T("Edit", "Изменить"), action: "edit", description: i18n.T("Change this server profile.", "Изменить профиль сервера.")},
		actionMenuItem{label: i18n.T("Delete", "Удалить"), action: "delete", description: i18n.T("Permanently remove this server profile.", "Безвозвратно удалить профиль сервера.")},
	)
	return newMenuModel(i18n.T("Server Actions", "Действия с сервером"), items, w, h)
}

func newManageMenuModel(w, h int, availability ...bool) *actionMenuModel {
	sessionsAvailable := len(availability) > 0 && availability[0]
	items := []list.Item{
		actionMenuItem{label: i18n.T("Groups", "Группы"), action: "groups", description: i18n.T("Create, rename, and remove server groups.", "Создать, переименовать или удалить группы серверов.")},
		actionMenuItem{label: i18n.T("Tags", "Теги"), action: "tags", description: i18n.T("Manage tags and apply them to selected servers.", "Управлять тегами и назначать их выбранным серверам.")},
		actionMenuItem{label: i18n.T("Command templates", "Шаблоны команд"), action: "templates", description: i18n.T("Manage reusable commands.", "Управлять повторно используемыми командами.")},
	}
	if sessionsAvailable {
		items = append(items, actionMenuItem{label: i18n.T("Sessions", "Сессии"), action: "sessions", description: i18n.T("Attach to or close tmux-backed SSH sessions.", "Подключиться к SSH-сессиям tmux или закрыть их.")})
	}
	items = append(items,
		actionMenuItem{label: i18n.T("Running tunnels", "Работающие туннели"), action: "tunnels", description: i18n.T("Inspect and stop tracked background tunnel processes.", "Просматривать и останавливать фоновые процессы туннелей.")},
		actionMenuItem{label: i18n.T("Settings", "Настройки"), action: "settings", description: i18n.T("Choose the interface language.", "Выбрать язык интерфейса.")},
		actionMenuItem{label: i18n.T("Import SSH config", "Импорт конфигурации SSH"), action: "import", description: i18n.T("Import profiles from ~/.ssh/config.", "Импортировать профили из ~/.ssh/config.")},
		actionMenuItem{label: i18n.T("Export", "Экспорт"), action: "export", description: i18n.T("Export server profiles.", "Экспортировать профили серверов.")},
		actionMenuItem{label: i18n.T("Vault: lock", "Заблокировать хранилище"), action: "vault_lock", description: i18n.T("Lock secrets for the current session.", "Заблокировать секреты текущей сессии.")},
		actionMenuItem{label: i18n.T("Vault: change password", "Сменить пароль хранилища"), action: "vault_change_pw", description: i18n.T("Change the password protecting stored secrets.", "Изменить пароль защиты сохранённых секретов.")},
	)
	return newMenuModel(i18n.T("Manage", "Управление"), items, w, h)
}

func newMenuModel(title string, items []list.Item, w, h int) *actionMenuModel {
	l := list.New(items, list.NewDefaultDelegate(), 34, len(items)+2)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = titleStyle
	return &actionMenuModel{list: l, title: title, width: w, height: h}
}

func (m *actionMenuModel) Update(msg tea.Msg) (*actionMenuModel, *string) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type != tea.KeyEnter {
			m.errorText = ""
		}
		switch msg.Type {
		case tea.KeyEsc:
			return m, nil
		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(actionMenuItem); ok {
				if reason := m.unavailableReason(item.action); reason != "" {
					m.errorText = reason
					return m, nil
				}
				return m, &item.action
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	_ = cmd
	return m, nil
}

func (m *actionMenuModel) View() string {
	selected, _ := m.list.SelectedItem().(actionMenuItem)
	breadcrumb := m.title
	status := i18n.Tf("%d actions", "Действий: %d", len(m.list.Items()))
	if m.serverAlias != "" {
		breadcrumb += " / " + m.serverAlias
		status = i18n.Tf("%d enabled forwards", "Включённых правил: %d", m.enabledForwards)
	}
	notification := m.errorText
	if notification == "" {
		notification = m.unavailableReason(selected.action)
	}
	body := func(width, height int) string {
		capacity := max(1, height-2)
		listCapacity := capacity
		if classifyShellContent(width) == sizeNarrow && selected.action != "" {
			listCapacity = max(1, capacity-4)
		}
		listLines := m.actionLines(listCapacity)
		if classifyShellContent(width) == sizeWide {
			leftWidth := width * 48 / 100
			rightWidth := width - leftWidth - 1
			detail := []string{dashboardSection(i18n.T("Selected action", "Выбранное действие")), "", selected.label, ""}
			detail = append(detail, wrapCells(selected.description, max(1, rightWidth-4))...)
			if m.serverAlias != "" {
				detail = append(detail, "", i18n.T("Server: ", "Сервер: ")+m.serverAlias, i18n.Tf("Enabled forwards: %d", "Включённых правил: %d", m.enabledForwards))
			}
			if reason := m.unavailableReason(selected.action); reason != "" {
				detail = append(detail, wrapCells(reason, max(1, rightWidth-4))...)
			}
			return joinPanelColumns(
				renderPaddedPanel(leftWidth, height, listLines), leftWidth,
				renderPaddedPanel(rightWidth, height, detail), rightWidth,
			)
		}
		if classifyShellContent(width) == sizeMedium {
			if selected, ok := m.list.SelectedItem().(actionMenuItem); ok && len(listLines) < height-4 {
				listLines = append(listLines, "", dashboardSection(i18n.T("Selected", "Выбрано")), selected.description)
			}
		}
		if classifyShellContent(width) == sizeNarrow && selected.action != "" {
			listLines = append(listLines, "", dashboardSection(i18n.T("Selected action", "Выбранное действие")))
			listLines = append(listLines, wrapCells(selected.description, max(1, width-4))...)
		}
		return renderPaddedPanel(width, height, listLines)
	}
	return renderScreenShell(screenShell{
		breadcrumb:   breadcrumb,
		status:       status,
		notification: notification,
		width:        m.width,
		height:       m.height,
		body:         body,
		footer: []helpItem{
			{Key: "↑/↓", Action: i18n.T("move", "перемещение")},
			{Key: "Enter", Action: i18n.T("select", "выбрать")},
			{Key: "Ctrl+H", Action: i18n.T("help", "справка")},
			{Key: "Esc", Action: i18n.T("back", "назад")},
		},
	})
}

func (m *actionMenuModel) actionLines(capacity int) []string {
	start, end := visibleServerRange(len(m.list.Items()), m.list.Index(), capacity)
	lines := make([]string, 0, capacity)
	for index := start; index < end; index++ {
		item, ok := m.list.Items()[index].(actionMenuItem)
		if !ok {
			continue
		}
		marker := "  "
		if index == m.list.Index() {
			marker = "> "
		}
		label := item.label
		if !m.loadingForwards && m.unavailableReason(item.action) != "" {
			label += i18n.T(" [unavailable]", " [недоступно]")
		}
		lines = append(lines, marker+label)
	}
	return lines
}
