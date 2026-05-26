package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <alias>",
	Short: "Delete a server profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]

		if !forceDelete {
			fmt.Printf("Are you sure you want to delete '%s'? (y/N): ", alias)
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := appDB.DeleteServer(alias); err != nil {
			return fmt.Errorf("delete server: %w", err)
		}

		fmt.Println("Deleted.")
		return nil
	},
}

var forceDelete bool

func init() {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Delete without confirmation")
}
