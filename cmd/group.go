package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var groupCmd = &cobra.Command{
	Use:   "group",
	Short: tr("Group management", "Управление группами"),
}

var groupListCmd = &cobra.Command{
	Use:   "list",
	Short: tr("List server groups", "Показать группы серверов"),
	RunE: func(cmd *cobra.Command, args []string) error {
		groups, err := appDB.GetGroups()
		if err != nil {
			return fmt.Errorf("%s: %w", tr("list groups", "получить список групп"), err)
		}

		if len(groups) == 0 {
			fmt.Println(tr("No groups. Use 'sshkeeper add --group <name>' to create one.", "Групп нет. Создайте группу командой 'sshkeeper add --group <name>'."))
			return nil
		}

		for _, g := range groups {
			fmt.Printf("  %s\n", g)
		}
		return nil
	},
}

var groupRenameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: tr("Rename a group (updates all servers in the group)", "Переименовать группу (обновить все серверы группы)"),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]

		if err := appDB.RenameGroup(oldName, newName); err != nil {
			return fmt.Errorf("%s: %w", tr("rename group", "переименовать группу"), err)
		}

		fmt.Println(trf("Group '%s' renamed to '%s'.", "Группа '%s' переименована в '%s'.", oldName, newName))
		return nil
	},
}

var groupDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: tr("Delete a group (removes group from all servers)", "Удалить группу у всех серверов"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if !forceFlag {
			fmt.Print(trf("Remove group '%s' from all servers? (y/N): ", "Удалить группу '%s' у всех серверов? (y/N): ", name))
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" {
				fmt.Println(tr("Cancelled.", "Отменено."))
				return nil
			}
		}

		if err := appDB.DeleteGroup(name); err != nil {
			return fmt.Errorf("%s: %w", tr("delete group", "удалить группу"), err)
		}

		fmt.Println(trf("Group '%s' removed from all servers.", "Группа '%s' удалена у всех серверов.", name))
		return nil
	},
}

var forceFlag bool

func init() {
	groupCmd.AddCommand(groupListCmd)
	groupCmd.AddCommand(groupRenameCmd)
	groupCmd.AddCommand(groupDeleteCmd)
	groupDeleteCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, tr("Delete without confirmation", "Удалить без подтверждения"))
}
