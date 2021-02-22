package cmd

import (
	"github.com/jenkins-x-plugins/jx-test/pkg/cmd/create"
	"github.com/jenkins-x-plugins/jx-test/pkg/cmd/gc"
	"github.com/jenkins-x-plugins/jx-test/pkg/cmd/version"
	"github.com/jenkins-x-plugins/jx-test/pkg/root"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
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
