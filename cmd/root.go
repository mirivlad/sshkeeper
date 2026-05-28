package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/mirivlad/sshkeeper/internal/config"
	"github.com/mirivlad/sshkeeper/internal/db"
	"github.com/mirivlad/sshkeeper/internal/vault"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	cfg   *config.Config
	appDB *db.DB
)

var rootCmd = &cobra.Command{
	Use:   "sshkeeper",
	Short: "sshkeeper — SSH connection manager",
	Long: `sshkeeper is a console SSH connection manager for Linux.
It manages server profiles, secrets, and provides a convenient way
to launch SSH sessions using the system OpenSSH client.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initApp)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(vaultCmd)
	rootCmd.AddCommand(sshConfigCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(groupCmd)
	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(runTemplateCmd)
}

func initApp() {
	var err error

	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	appDB, err = db.Open(cfg.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}

	// Handle vault: create on first run, unlock on subsequent runs
	vaultPath := config.VaultPath(cfg.DataDir)
	v := vault.New(vaultPath)

	if !vault.Exists(vaultPath) {
		// First run — create vault
		fmt.Println("Welcome to sshkeeper!")
		fmt.Println("No vault found. Let's create one.")
		fmt.Println()

		for {
			fmt.Print("Create master password: ")
			pw1, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}

			if len(pw1) == 0 {
				fmt.Println("Password cannot be empty. Try again.")
				continue
			}

			fmt.Print("Repeat master password: ")
			pw2, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}

			if string(pw1) != string(pw2) {
				fmt.Println("Passwords do not match. Try again.")
				continue
			}

			if err := vault.Create(vaultPath, string(pw1)); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating vault: %v\n", err)
				os.Exit(1)
			}

			// Unlock immediately after creation
			if err := v.Unlock(string(pw1)); err != nil {
				fmt.Fprintf(os.Stderr, "Error unlocking vault: %v\n", err)
				os.Exit(1)
			}

			vaultInstance = v
			fmt.Println()
			fmt.Println("Vault created and unlocked. You're ready to go!")
			fmt.Println()
			break
		}
	} else {
		// Vault exists — unlock only for commands that may need secrets.
		if !commandRequiresStartupVaultUnlock(os.Args[1:]) {
			vaultInstance = v
			return
		}

		for attempts := 0; attempts < 3; attempts++ {
			fmt.Print("Master password: ")
			pw, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
				os.Exit(1)
			}

			if err := v.Unlock(string(pw)); err != nil {
				remaining := 2 - attempts
				if remaining > 0 {
					fmt.Printf("Invalid password. %d attempts remaining.\n", remaining)
					continue
				}
				fmt.Fprintf(os.Stderr, "Too many failed attempts. Run 'sshkeeper vault unlock' to try again.\n")
				os.Exit(1)
			}

			vaultInstance = v
			fmt.Println("Vault unlocked.")
			fmt.Println()
			return
		}
	}
}

func commandRequiresStartupVaultUnlock(args []string) bool {
	if len(args) == 0 {
		return true
	}

	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return false
		}
	}

	switch args[0] {
	case "connect", "c", "run", "run-template", "test", "add", "edit", "delete":
		return true
	default:
		return false
	}
}
