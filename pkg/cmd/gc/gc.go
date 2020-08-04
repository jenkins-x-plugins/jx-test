package gc

import (
	"fmt"
	"time"

	"github.com/jenkins-x/jx-helpers/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/cli"
	"github.com/jenkins-x/jx-helpers/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-test/pkg/root"
	"github.com/jenkins-x/jx-test/pkg/testclients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Namespace     string
	Duration      time.Duration
	Tests         []v1alpha1.TestRun
	TestClient    versioned.Interface
	CommandRunner cmdrunner.CommandRunner
	GitClient     gitclient.Interface
	Errors        []error
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

// Validate checks everything is configured correctly and we can lazy create any missing clients
func (o *Options) Validate() error {
	var err error
	o.TestClient, o.Namespace, err = testclients.LazyCreateClient(o.TestClient, o.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to create test client")
	}
	if o.CommandRunner == nil {
		o.CommandRunner = cmdrunner.DefaultCommandRunner
	}
	if o.GitClient == nil {
		o.GitClient = cli.NewCLIClient("", o.CommandRunner)
	}
	return nil
}

// Run implements the command
func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate setup")
	}

	testList, err := o.TestClient.JxtestV1alpha1().TestRuns(o.Namespace).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to list TestRun instances in namespace %s", o.Namespace)
	}
	o.Tests = testList.Items
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
	if now.After(ttlExceededDate) {
		return true
	}
	return false
}

// ShouldDeleteDueToNewerRun returns true if a testRun with a higher build number exists
func (o *Options) ShouldDeleteDueToNewerRun(testRun *v1alpha1.TestRun, testRuns []v1alpha1.TestRun) bool {
	currentBuildNumber := testRun.Spec.BuildNumber
	if currentBuildNumber <= 0 {
		log.Logger().Warnf("test %s does not have a spec.buildNumber populated", testRun.Name)
		return false
	}

	testKind := testRun.Spec.TestKind()

	for _, ec := range testRuns {
		existingTestRun := ec

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
	name := testRun.Name
	ns := o.Namespace
	log.Logger().Infof("removing TestsRun resources for %s in namespace %s", termcolor.ColorInfo(name), termcolor.ColorInfo(ns))

	bin := testRun.Spec.RemoveScript
	if bin == "" {
		bin = "bin/destroy.sh"
		log.Logger().Warnf("the TestRun %s does not have a spec.removeScript so using default: %s", name, bin)
	}

	testURL := testRun.Spec.TestSource.URL
	if testURL == "" {
		o.Errors = append(o.Errors, errors.Errorf("TestRun %s has no spec.testSource.url", name))
		return
	}

	dir, err := gitclient.CloneToDir(o.GitClient, testURL, "")
	if err != nil {
		o.Errors = append(o.Errors, errors.Wrapf(err, "failed to clone %s for TestRun %s in namespace %s", testURL, name, ns))
		return
	}

	c := &cmdrunner.Command{
		Dir:  dir,
		Name: bin,
		Env:  testRun.Spec.Env,
	}
	_, err = o.CommandRunner(c)
	if err != nil {
		o.Errors = append(o.Errors, errors.Wrapf(err, "failed to run %s in git clone of %s for TestRun %s in namespace %s", bin, testURL, name, ns))
		return
	}

	err = o.TestClient.JxtestV1alpha1().TestRuns(ns).Delete(name, nil)
	if err != nil {
		o.Errors = append(o.Errors, errors.Wrapf(err, "failed to delete TestRun %s in namespace %s", name, ns))
	}
}
