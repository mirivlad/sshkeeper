package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/vault"
	"golang.org/x/term"
)

var vaultInstance *vault.Vault

func getOrCreateVault() *vault.Vault {
	if vaultInstance == nil {
		vaultInstance = vault.New(config.VaultPath(cfg.DataDir))
	}
	return vaultInstance
}

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Vault management commands",
}

var vaultUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock the vault with master password",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := getOrCreateVault()

		if v.IsUnlocked() {
			fmt.Println("Vault is already unlocked.")
			return nil
		}

		vaultPath := config.VaultPath(cfg.DataDir)

		// Check if vault exists and has content
		info, err := os.Stat(vaultPath)
		if os.IsNotExist(err) || info.Size() == 0 {
			// New vault - create with master password
			fmt.Print("Create master password: ")
			pw1, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}

			if len(pw1) == 0 {
				return fmt.Errorf("password cannot be empty")
			}

			fmt.Print("Repeat master password: ")
			pw2, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}

			if string(pw1) != string(pw2) {
				return fmt.Errorf("passwords do not match")
			}

			if err := vault.Create(vaultPath, string(pw1)); err != nil {
				return fmt.Errorf("create vault: %w", err)
			}

			// Unlock immediately
			if err := v.Unlock(string(pw1)); err != nil {
				return fmt.Errorf("unlock vault: %w", err)
			}

			fmt.Println("Vault created and unlocked.")
			return nil
		}

		// Unlock existing vault
		fmt.Print("Master password: ")
		pw, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}

		if err := v.Unlock(string(pw)); err != nil {
			return fmt.Errorf("unlock vault: %w", err)
		}

		fmt.Println("Vault unlocked.")
		return nil
	},
}

var vaultLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock the vault",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := getOrCreateVault()
		v.Lock()
		fmt.Println("Vault locked.")
		return nil
	},
}

var vaultStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show vault status",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := getOrCreateVault()
		if v.IsUnlocked() {
			fmt.Println("Vault: unlocked")
		} else {
			fmt.Println("Vault: locked")
		}
		return nil
	},
}

var vaultChangePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Change master password",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := getOrCreateVault()

		if !v.IsUnlocked() {
			return fmt.Errorf("vault is locked. Unlock first with 'sshkeeper vault unlock'")
		}

		fmt.Print("New master password: ")
		pw1, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}

		if len(pw1) == 0 {
			return fmt.Errorf("password cannot be empty")
		}

		fmt.Print("Repeat new master password: ")
		pw2, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}

		if string(pw1) != string(pw2) {
			return fmt.Errorf("passwords do not match")
		}

		if err := v.ChangePassword(string(pw1)); err != nil {
			return fmt.Errorf("change password: %w", err)
		}

		fmt.Println("Master password changed.")
		return nil
	},
}

func init() {
	vaultCmd.AddCommand(vaultUnlockCmd)
	vaultCmd.AddCommand(vaultLockCmd)
	vaultCmd.AddCommand(vaultStatusCmd)
	vaultCmd.AddCommand(vaultChangePasswordCmd)
}
