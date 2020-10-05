package create

import (
	"fmt"
	"os"
	"strings"

	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/naming"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-test/pkg/root"
	"github.com/jenkins-x/jx-test/pkg/testclients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	cmdLong = templates.LongDesc(`
		Garbage collects test resources
`)

	cmdExample = templates.Examples(`
		%s create --test-url https://github.com/myorg/mytest.git
	`)
)

// Options the options for the command
type Options struct {
	Namespace    string
	TestGitURL   string
	RemoveScript string
	Env          []string
	TestClient   versioned.Interface
	TestRun      *v1alpha1.TestRun
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
	cmd.Flags().StringVarP(&o.Namespace, "ns", "n", "", "the namespace to filter the TestRun resources")
	cmd.Flags().StringVarP(&o.TestGitURL, "test-url", "u", "", "the git URL of the test case which is used to remove the resources")
	cmd.Flags().StringVarP(&o.RemoveScript, "remove-script", "s", "bin/destroy.sh", "the script in the test git url used to remove the resources")
	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate")
	}

	test := &v1alpha1.TestRun{}
	err = o.PopulateTest(test)
	if err != nil {
		return errors.Wrapf(err, "failed to populate the TestRun resource")
	}

	o.TestRun, err = o.TestClient.JxtestV1alpha1().TestRuns(o.Namespace).Create(test)
	if err != nil {
		return errors.Wrapf(err, "failed to create the TestRun CRD")
	}
	return nil
}

func (o *Options) Validate() error {
	if o.TestGitURL == "" {
		return options.MissingOption("test-url")
	}
	if o.Env == nil {
		o.Env = os.Environ()
	}
	var err error
	o.TestClient, o.Namespace, err = testclients.LazyCreateClient(o.TestClient, o.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to create test client")
	}
	return nil
}

func (o *Options) PopulateTest(test *v1alpha1.TestRun) error {
	test.Spec.TestSource.URL = o.TestGitURL
	test.Spec.RemoveScript = o.RemoveScript

	if test.Spec.Env == nil {
		test.Spec.Env = map[string]string{}
	}
	for _, e := range o.Env {
		values := strings.SplitN(e, "=", 2)
		if len(values) == 2 {
			test.Spec.Env[values[0]] = values[1]
		}
	}
	err := test.Spec.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate TestRun spec")
	}
	if test.Name == "" {
		gitInfo, err := giturl.ParseGitURL(o.TestGitURL)
		if err != nil {
			return errors.Wrapf(err, "failed to parse test git url %s", o.TestGitURL)
		}
		names := []string{gitInfo.Organisation, gitInfo.Name}
		test.Name = naming.ToValidName(strings.Join(names, "-"))
	}
	return nil
}
