package create

import (
	"context"
	"fmt"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/pipelinectx"
	"github.com/jenkins-x/jx-test/pkg/root"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"strings"
)

var (
	info = termcolor.ColorInfo

	cmdLong = templates.LongDesc(`
		Garbage collects test resources
`)

	cmdExample = templates.Examples(`
		%s create --test-url https://github.com/myorg/mytest.git
	`)
)

// Options the options for the command
type Options struct {
	pipelinectx.Options
	File          string
	Kind          string
	CommandRunner cmdrunner.CommandRunner
}

// NewCmdCreate creates a command object for the command
func NewCmdCreate() (*cobra.Command, *Options) {
	o := &Options{}

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new TestRun resource to record the test case resources",
		Long:    cmdLong,
		Example: fmt.Sprintf(cmdExample, root.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.TODO()
	}
	err := o.EnvironmentDefaults(ctx)
	if err != nil {
		log.Logger().Warnf("failed to default env vars: %s", err.Error())
	}

	o.Options.AddFlags(cmd)
	cmd.Flags().StringVarP(&o.File, "file", "f", "", "the template file to create")
	cmd.Flags().StringVarP(&o.Kind, "kind", "", "terraform", "the kubernetes kind of the resource ot create/delete")
	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate")
	}

	log.Logger().Infof("resource: %s", info(o.ResourceName))
	log.Logger().Infof("labels: %v", o.Labels)

	// lets delete any old resources

	if len(o.Labels) == 0 {
		return errors.Errorf("no labels could be created")
	}
	buf := &strings.Builder{}
	for k, v := range o.Labels {
		if buf.Len() > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(v)
	}

	selector := buf.String()

	c := &cmdrunner.Command{
		Name: "kubectl",
		Args: []string{"delete", o.Kind, "-l", selector},
	}
	_, err = o.CommandRunner(c)
	if err != nil {
		return errors.Wrapf(err, "failed to run %s", c.CLI())
	}
	log.Logger().Infof("removed %s resources with selector %s", info(o.Kind), info(selector))
	return nil
}

func (o *Options) Validate() error {
	err := o.Options.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate pipeline options")
	}

	if o.File == "" {
		return options.MissingOption("file")
	}

	if o.CommandRunner == nil {
		o.CommandRunner = cmdrunner.DefaultCommandRunner
	}
	return nil
}

// ShouldDeleteDueToNewerRun returns true if a testRun with a higher build number exists
func (o *Options) MarkOldRunsAsDelete(currentTestRun *v1alpha1.TestRun, testRuns []v1alpha1.TestRun) error {
	/*
		testKind := currentTestRun.Spec.TestKind()

		for i := 0; i < len(testRuns); i++ {
			testRun := &testRuns[i]

			// check for same branch, context and trigger source  URL
			existingTestKind := testRun.Spec.TestKind()
			if existingTestKind == testKind {
				err := testclients.MarkDeleted(o.TestClient, o.Namespace, testRun)
				if err != nil {
					return errors.Wrapf(err, "failed to delete currentTestRun %s", testRun.Name)
				}
			}
		}

	*/
	return nil
}
