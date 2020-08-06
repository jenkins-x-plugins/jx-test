package deletecmd_test

import (
	"testing"

	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-test/pkg/cmd/create"
	"github.com/jenkins-x/jx-test/pkg/cmd/deletecmd"
	"github.com/jenkins-x/jx-test/pkg/testruntesters"
	"github.com/stretchr/testify/require"
)

func TestDelete(t *testing.T) {
	testClient := fake.NewSimpleClientset()
	ns := "jx"
	testGitURL := "https://github.com/myorg/env-pr-1243-5-bdd-thingy-dev.git"
	env := []string{
		"BRANCH_NAME=PR-1234",
		"BUILD_NUMBER=3",
		"PIPELINE_CONTEXT=gke-terraform-vault",
		"SOURCE_URL=https://github.com/jenkins-x/jx3-versions",
	}

	_, do := deletecmd.NewCmdDelete()
	do.TestGitURL = testGitURL
	do.Namespace = ns
	do.TestClient = testClient
	do.Dir = "test_data"

	// check we fail when no TestRuns
	err := do.Run()
	require.Error(t, err, "should have failed as we have no TestRuns yet")

	// lets create a TestRun
	_, co := create.NewCmdCreate()
	co.TestGitURL = testGitURL
	co.Namespace = ns
	co.TestClient = testClient
	co.Env = env
	err = co.Run()
	require.NoError(t, err, "failed to run create command")

	// lets mark the TestRun as keep
	testruntesters.RequireModifyTestRun(t, do.TestClient, do.Namespace, testGitURL, func(tr *v1alpha1.TestRun) error {
		tr.Spec.Keep = true
		return nil
	})

	// if we try delete it then it should not do anything....
	err = do.Run()
	require.NoError(t, err, "failed to delete the TestRun")

	// lets mark the TestRun as not keep
	testruntesters.RequireModifyTestRun(t, do.TestClient, do.Namespace, testGitURL, func(tr *v1alpha1.TestRun) error {
		tr.Spec.Keep = false
		return nil
	})

	// check we delete it...
	err = do.Run()
	require.NoError(t, err, "failed to delete the TestRun")
}
