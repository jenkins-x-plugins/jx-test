package gc

import (
	"fmt"
	"github.com/jenkins-x/jx-test/pkg/testclients"
	"time"

	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/root"
	"github.com/jenkins-x/jx-test/pkg/testclients/deleter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	cmdLong = templates.LongDesc(`
		Garbage collects test resources
`)

	cmdExample = templates.Examples(`
		%s gc
	`)
)

// Options the options for the command
type Options struct {
	deleter.Options
	Duration time.Duration
	Tests    []v1alpha1.TestRun
	Errors   []error
}

// NewCmdGC creates a command object for the command
func NewCmdGC() (*cobra.Command, *Options) {
	o := &Options{}

	cmd := &cobra.Command{
		Use:     "gc",
		Short:   "Garbage collects test resources",
		Long:    cmdLong,
		Example: fmt.Sprintf(cmdExample, root.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&o.Namespace, "ns", "n", "", "the namespace to filter the TestRun resources")
	cmd.Flags().DurationVarP(&o.Duration, "duration", "d", 2*time.Hour, "How long before the test is deleted if it does not have a delete flag")
	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {
	err := o.Options.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate setup")
	}

	o.Tests, err = testclients.ListTestRuns(o.TestClient, o.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to find TestRuns")
	}
	return o.GC()
}

func (o *Options) GC() error {
	for i := range o.Tests {
		c := &o.Tests[i]
		if c.Spec.Keep {
			continue
		}
		if c.Spec.Delete {
			o.DeleteTestRun(c)
			continue
		}

		if o.ShouldDeleteOlderThanDuration(c) {
			o.DeleteTestRun(c)
			continue
		}

		if o.ShouldDeleteDueToNewerRun(c, o.Tests) {
			o.DeleteTestRun(c)
		}
	}

	for _, e := range o.Errors {
		log.Logger().Error(e.Error())
	}
	return nil
}

// ShouldDeleteOlderThanDuration returns true if the testRun is older than the delete duration and does not have a keep label
func (o *Options) ShouldDeleteOlderThanDuration(testRun *v1alpha1.TestRun) bool {
	if testRun.Spec.Keep {
		return false
	}
	if testRun.Spec.Delete {
		return true
	}

	created := testRun.CreationTimestamp.Time
	ttlExceededDate := created.Add(o.Duration)
	now := time.Now()
	return now.After(ttlExceededDate)
}

// ShouldDeleteDueToNewerRun returns true if a testRun with a higher build number exists
func (o *Options) ShouldDeleteDueToNewerRun(testRun *v1alpha1.TestRun, testRuns []v1alpha1.TestRun) bool {
	currentBuildNumber := testRun.Spec.BuildNumber
	if currentBuildNumber <= 0 {
		log.Logger().Warnf("test %s does not have a spec.buildNumber populated", testRun.Name)
		return false
	}

	testKind := testRun.Spec.TestKind()
	for i := 0; i < len(testRuns); i++ {
		existingTestRun := testRuns[i]

		// check for same branch, context and trigger source  URL
		existingTestKind := existingTestRun.Spec.TestKind()
		if existingTestKind == testKind {
			existingBuildNumber := existingTestRun.Spec.BuildNumber
			// Delete the older build
			if existingBuildNumber > 0 && currentBuildNumber < existingBuildNumber {
				return true
			}
		}
	}
	return false
}

// DeleteTestRun deletes the test run resources and the CRD
func (o *Options) DeleteTestRun(testRun *v1alpha1.TestRun) {
	err := o.Options.DoDelete(testRun, "", "")
	if err != nil {
		o.Errors = append(o.Errors, err)
		return
	}
}
