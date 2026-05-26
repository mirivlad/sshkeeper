package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var groupCmd = &cobra.Command{
	Use:   "group",
	Short: "Group management",
}

var groupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List server groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		groups, err := appDB.GetGroups()
		if err != nil {
			return fmt.Errorf("list groups: %w", err)
		}

		if len(groups) == 0 {
			fmt.Println("No groups. Use 'sshkeeper add --group <name>' to create one.")
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
	Short: "Rename a group (updates all servers in the group)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]

		if err := appDB.RenameGroup(oldName, newName); err != nil {
			return fmt.Errorf("rename group: %w", err)
		}

		fmt.Printf("Group '%s' renamed to '%s'.\n", oldName, newName)
		return nil
	},
}

var groupDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a group (removes group from all servers)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if !forceFlag {
			fmt.Printf("Remove group '%s' from all servers? (y/N): ", name)
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := appDB.DeleteGroup(name); err != nil {
			return fmt.Errorf("delete group: %w", err)
		}

		fmt.Printf("Group '%s' removed from all servers.\n", name)
		return nil
	},
}

var forceFlag bool

func init() {
	groupCmd.AddCommand(groupListCmd)
	groupCmd.AddCommand(groupRenameCmd)
	groupCmd.AddCommand(groupDeleteCmd)
	groupDeleteCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Delete without confirmation")
}
