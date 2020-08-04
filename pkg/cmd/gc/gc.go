package gc

import (
	"fmt"
	"time"

	"github.com/jenkins-x/jx-helpers/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/templates"
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
	CommandRunner cmdrunner.CommandRunner
	Tests         []v1alpha1.TestRun
	TestClient    versioned.Interface
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

// Run implements the command
func (o *Options) Run() error {
	var err error
	o.TestClient, o.Namespace, err = testclients.LazyCreateClient(o.TestClient, o.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to create test client")
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
			o.deleteTest(c)
			continue
		}

		if o.ShouldDeleteOlderThanDuration(c) {
			o.deleteTest(c)
			continue
		}

		if o.ShouldDeleteDueToNewerRun(c, o.Tests) {
			o.deleteTest(c)
		}
	}
	return nil
}

// ShouldDeleteOlderThanDuration returns true if the cluster is older than the delete duration and does not have a keep label
func (o *Options) ShouldDeleteOlderThanDuration(cluster *v1alpha1.TestRun) bool {
	if cluster.Spec.Keep {
		return false
	}
	if cluster.Spec.Delete {
		return true
	}

	created := cluster.CreationTimestamp.Time
	ttlExceededDate := created.Add(o.Duration)
	now := time.Now()
	if now.After(ttlExceededDate) {
		return true
	}
	return false
}

// ShouldDeleteDueToNewerRun returns true if a cluster with a higher build number exists
func (o *Options) ShouldDeleteDueToNewerRun(cluster *v1alpha1.TestRun, clusters []v1alpha1.TestRun) bool {
	currentBuildNumber := cluster.Spec.BuildNumber
	if currentBuildNumber <= 0 {
		log.Logger().Warnf("test %s does not have a spec.buildNumber populated", cluster.Name)
		return false
	}

	testKind := cluster.Spec.TestKind()

	for _, ec := range clusters {
		existingCluster := ec
		// Check for same branch, context and strigger source  URL
		existingTestKind := existingCluster.Spec.TestKind()
		if existingTestKind == testKind {
			existingBuildNumber := existingCluster.Spec.BuildNumber
			// Delete the older build
			if existingBuildNumber > 0 && currentBuildNumber < existingBuildNumber {
				return true
			}
		}
	}
	return false
}

func (o *Options) deleteTest(c *v1alpha1.TestRun) {
	log.Logger().Infof("removing test %s", termcolor.ColorInfo(c.Name))

	err := o.TestClient.JxtestV1alpha1().TestRuns(o.Namespace).Delete(c.Name, nil)
	if err != nil {
		o.Errors = append(o.Errors, err)
	}
}
