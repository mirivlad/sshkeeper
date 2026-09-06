package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is replaced at link time by build.sh/release.sh.
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print sshkeeper version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "sshkeeper %s\n", Version)
			return err
		},
	}
}

var versionCmd = newVersionCmd()
