package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <alias>",
	Short: tr("Delete a server profile", "Удалить профиль сервера"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]

		if !forceDelete {
			fmt.Print(trf("Are you sure you want to delete '%s'? (y/N): ", "Удалить '%s'? (y/N): ", alias))
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println(tr("Cancelled.", "Отменено."))
				return nil
			}
		}

		if err := appDB.DeleteServer(alias); err != nil {
			return fmt.Errorf("%s: %w", tr("delete server", "удалить сервер"), err)
		}

		// Clean up vault secrets for this server
		v := getOrCreateVault()
		if v.IsUnlocked() {
			cleanupServerSecrets(v, alias)
			if err := v.Save(); err != nil {
				return fmt.Errorf("%s: %w", tr("save vault after cleanup", "сохранить хранилище после очистки"), err)
			}
		}

		fmt.Println(tr("Deleted.", "Удалено."))
		return nil
	},
}

var forceDelete bool

func init() {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, tr("Delete without confirmation", "Удалить без подтверждения"))
}
