package cmd

import (
	"github.com/jenkins-x/jx-helpers/pkg/cobras"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-test/pkg/cmd/create"
	"github.com/jenkins-x/jx-test/pkg/cmd/gc"
	"github.com/jenkins-x/jx-test/pkg/cmd/version"
	"github.com/jenkins-x/jx-test/pkg/root"
	"github.com/spf13/cobra"
)

// Main creates the new command
func Main() *cobra.Command {
	cmd := &cobra.Command{
		Use:   root.TopLevelCommand,
		Short: "Test commands",
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				log.Logger().Errorf(err.Error())
			}
		},
	}
	cmd.AddCommand(cobras.SplitCommand(create.NewCmdCreate()))
	cmd.AddCommand(cobras.SplitCommand(gc.NewCmdGC()))
	cmd.AddCommand(cobras.SplitCommand(version.NewCmdVersion()))
	return cmd
}
