package cmd

import (
	"fmt"

	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Global command template management",
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List global command templates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		templates, err := appDB.ListCommandTemplates()
		if err != nil {
			return fmt.Errorf("list templates: %w", err)
		}

		if len(templates) == 0 {
			fmt.Println("No command templates.")
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
	Short: "Add a global command template",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		t := &model.CommandTemplate{Name: args[0], Command: args[1]}
		if err := appDB.CreateCommandTemplate(t); err != nil {
			return fmt.Errorf("add template: %w", err)
		}

		fmt.Println("Template added.")
		return nil
	},
}

var templateEditCmd = &cobra.Command{
	Use:   "edit <old-name> <name> <command>",
	Short: "Edit a global command template",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		t := &model.CommandTemplate{Name: args[1], Command: args[2]}
		if err := appDB.UpdateCommandTemplate(args[0], t); err != nil {
			return fmt.Errorf("edit template: %w", err)
		}

		fmt.Println("Template saved.")
		return nil
	},
}

var templateDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a global command template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := appDB.DeleteCommandTemplate(args[0]); err != nil {
			return fmt.Errorf("delete template: %w", err)
		}

		fmt.Println("Template deleted.")
		return nil
	},
}

var runTemplateCmd = &cobra.Command{
	Use:   "run-template <alias> <template>",
	Short: "Run a global command template on a server",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		templateName := args[1]

		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		template, err := appDB.GetCommandTemplate(templateName)
		if err != nil {
			return fmt.Errorf("template not found: %s", templateName)
		}

		fmt.Printf("Running '%s' on %s...\n", template.Command, alias)
		return runCommandOnServer(server, template.Command)
	},
}

func init() {
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateAddCmd)
	templateCmd.AddCommand(templateEditCmd)
	templateCmd.AddCommand(templateDeleteCmd)
}
