package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Command template management",
}

var templateListCmd = &cobra.Command{
	Use:   "list <alias>",
	Short: "List command templates for a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		_, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		templates, err := appDB.GetCommandTemplates(alias)
		if err != nil {
			return fmt.Errorf("list templates: %w", err)
		}

		if len(templates) == 0 {
			fmt.Println("No command templates.")
			return nil
		}

		for _, t := range templates {
			fmt.Printf("  %-15s %s\n", t.Name, t.Command)
		}
		return nil
	},
}

var templateAddCmd = &cobra.Command{
	Use:   "add <alias> <name> <command>",
	Short: "Add a command template",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		name := args[1]
		command := args[2]

		server, err := appDB.GetServer(alias)
		if err != nil {
			return fmt.Errorf("server not found: %s", alias)
		}

		if err := appDB.AddCommandTemplate(server.ID, name, command); err != nil {
			return fmt.Errorf("add template: %w", err)
		}

		fmt.Println("Template added.")
		return nil
	},
}

var runTemplateCmd = &cobra.Command{
	Use:   "run-template <alias> <template>",
	Short: "Run a command template on a server",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		templateName := args[1]

		templates, err := appDB.GetCommandTemplates(alias)
		if err != nil {
			return fmt.Errorf("list templates: %w", err)
		}
		if len(templates) == 0 {
			return fmt.Errorf("server not found or no templates: %s", alias)
		}

		var command string
		for _, t := range templates {
			if t.Name == templateName {
				command = t.Command
				break
			}
		}

		if command == "" {
			return fmt.Errorf("template not found: %s", templateName)
		}

		fmt.Printf("Running '%s' on %s...\n", command, alias)
		return nil
	},
}

func init() {
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateAddCmd)
}
