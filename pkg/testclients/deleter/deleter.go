package deleter

import (
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/cli"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-test/pkg/testclients"
	"github.com/pkg/errors"
)

type Options struct {
	Namespace     string
	TestClient    versioned.Interface
	CommandRunner cmdrunner.CommandRunner
	GitClient     gitclient.Interface
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

// DoDelete deletes the test run resources and the CRD
// can pass in the dir and bin script if they already exist locally
func (o *Options) MarkDeleted(testRun *v1alpha1.TestRun, dir, bin string) error {
	return testclients.MarkDeleted(o.TestClient, o.Namespace, testRun)
}

// DoDelete deletes the test run resources and the CRD
// can pass in the dir and bin script if they already exist locally
func (o *Options) DoDelete(testRun *v1alpha1.TestRun, dir, bin string) error {
	name := testRun.Name
	ns := o.Namespace
	log.Logger().Infof("removing TestsRun resources for %s in namespace %s", termcolor.ColorInfo(name), termcolor.ColorInfo(ns))

	if bin == "" {
		bin = "bin/destroy.sh"
		log.Logger().Warnf("the TestRun %s does not have a spec.removeScript so using default: %s", name, bin)
	}

	testURL := testRun.Spec.TestSource.URL
	if testURL == "" {
		return errors.Errorf("TestRun %s has no spec.testSource.url", name)
	}
	var err error

	if dir == "" {
		dir, err = gitclient.CloneToDir(o.GitClient, testURL, "")
		if err != nil {
			return errors.Wrapf(err, "failed to clone %s for TestRun %s in namespace %s", testURL, name, ns)
		}
	}

	c := &cmdrunner.Command{
		Dir:  dir,
		Name: bin,
		Env:  testRun.Spec.Env,
	}
	_, err = o.CommandRunner(c)
	if err != nil {
		return errors.Wrapf(err, "failed to run %s in git clone of %s for TestRun %s in namespace %s", bin, testURL, name, ns)
	}

	err = o.TestClient.JxtestV1alpha1().TestRuns(ns).Delete(name, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to delete TestRun %s in namespace %s", name, ns)
	}
	return nil
}
