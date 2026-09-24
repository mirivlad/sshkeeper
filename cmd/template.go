package cmd

import (
	"fmt"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: tr("Global command template management", "Управление общими шаблонами команд"),
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: tr("List global command templates", "Показать общие шаблоны команд"),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		templates, err := appDB.ListCommandTemplates()
		if err != nil {
			return fmt.Errorf("%s: %w", tr("list templates", "получить список шаблонов"), err)
		}

		if len(templates) == 0 {
			fmt.Println(tr("No command templates.", "Шаблонов команд нет."))
			return nil
		}

		for _, t := range templates {
			fmt.Printf("  %-20s %s\n", t.Name, t.Command)
		}
		return nil
	},
}

var templateAddCmd = &cobra.Command{
	Use:   "add <name> <command>",
	Short: tr("Add a global command template", "Добавить общий шаблон команды"),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		t := &model.CommandTemplate{Name: args[0], Command: args[1]}
		if err := appDB.CreateCommandTemplate(t); err != nil {
			return fmt.Errorf("%s: %w", tr("add template", "добавить шаблон"), err)
		}

		fmt.Println(tr("Template added.", "Шаблон добавлен."))
		return nil
	},
}

var templateEditCmd = &cobra.Command{
	Use:   "edit <old-name> <name> <command>",
	Short: tr("Edit a global command template", "Изменить общий шаблон команды"),
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		t := &model.CommandTemplate{Name: args[1], Command: args[2]}
		if err := appDB.UpdateCommandTemplate(args[0], t); err != nil {
			return fmt.Errorf("%s: %w", tr("edit template", "изменить шаблон"), err)
		}

		fmt.Println(tr("Template saved.", "Шаблон сохранён."))
		return nil
	},
}

var templateDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: tr("Delete a global command template", "Удалить общий шаблон команды"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := appDB.DeleteCommandTemplate(args[0]); err != nil {
			return fmt.Errorf("%s: %w", tr("delete template", "удалить шаблон"), err)
		}

		fmt.Println(tr("Template deleted.", "Шаблон удалён."))
		return nil
	},
}

var runTemplateCmd = &cobra.Command{
	Use:   "run-template <alias> <template>",
	Short: tr("Run a global command template on a server", "Выполнить шаблон команды на сервере"),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		templateName := args[1]

		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("%s", trf("server not found: %s", "сервер не найден: %s", alias))
		}

		template, err := appDB.GetCommandTemplate(templateName)
		if err != nil {
			return fmt.Errorf("%s", trf("template not found: %s", "шаблон не найден: %s", templateName))
		}

		fmt.Printf(tr("Running '%s' on %s...\n", "Выполнение '%s' на %s...\n"), template.Command, alias)
		return runCommandOnServer(server, template.Command)
	},
}

func init() {
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateAddCmd)
	templateCmd.AddCommand(templateEditCmd)
	templateCmd.AddCommand(templateDeleteCmd)
}
