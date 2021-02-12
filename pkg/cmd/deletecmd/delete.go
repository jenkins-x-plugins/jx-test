package deletecmd

import (
	"fmt"

	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/jenkins-x/jx-test/pkg/root"
	"github.com/jenkins-x/jx-test/pkg/testclients/deleter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	cmdLong = templates.LongDesc(`
		"Deletes the TestRun cloud resources and CRD unless its been marked as keep: true
`)

	cmdExample = templates.Examples(`
		%s delete --test-url $GITOPS_REPO --dir=. --script=bin/destroy.sh	
	`)
)

// Options the options for the command
type Options struct {
	deleter.Options
	Dir          string
	TestGitURL   string
	RemoveScript string
}

// NewCmdGC creates a command object for the command
func NewCmdDelete() (*cobra.Command, *Options) {
	o := &Options{}

	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Deletes the TestRun cloud resources and CRD unless its been marked as keep: true",
		Aliases: []string{"rm", "remove", "del"},
		Long:    cmdLong,
		Example: fmt.Sprintf(cmdExample, root.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&o.Namespace, "ns", "n", "", "the namespace to filter the TestRun resources")
	cmd.Flags().StringVarP(&o.TestGitURL, "test-url", "u", "", "the git URL of the test case which is used to remove the resources")
	cmd.Flags().StringVarP(&o.Dir, "dir", "d", ".", "the directory of the git clone")
	cmd.Flags().StringVarP(&o.RemoveScript, "script", "s", "", "the script in the test git url used to remove the resources")
	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {
	if o.TestGitURL == "" {
		return options.MissingOption("test-url")
	}
	err := o.Options.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate setup")
	}

	testList, err := o.TestClient.JxtestV1alpha1().TestRuns(o.Namespace).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to list TestRun instances in namespace %s", o.Namespace)
	}
	for i := 0; i < len(testList.Items); i++ {
		tr := testList.Items[i]
		if tr.Spec.TestSource.URL != o.TestGitURL {
			continue
		}
		if tr.Spec.Keep {
			log.Logger().Infof("not marking TestRun %s in namespace %s as deleted as it is marked as KEEP", termcolor.ColorInfo(tr.Name), termcolor.ColorInfo(o.Namespace))
			return nil
		}
		if tr.Spec.Delete {
			log.Logger().Infof("the TestRun %s in namespace %s is already marked as DELETE", termcolor.ColorInfo(tr.Name), termcolor.ColorInfo(o.Namespace))
			return nil
		}

		err = o.Options.MarkDeleted(&tr, o.Dir, o.RemoveScript)
		if err != nil {
			return errors.Wrapf(err, "failed to mark TestRun %s in namespace %s as deleted", tr.Name, o.Namespace)
		}
		log.Logger().Infof("marked TestRun %s in namespace %s as deleted", termcolor.ColorInfo(tr.Name), termcolor.ColorInfo(o.Namespace))
		return nil
	}
	return errors.Errorf("could not find a TestRun in namespace %s which has spec.testSource.url = %s", o.Namespace, o.TestGitURL)
}
