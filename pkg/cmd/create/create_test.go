package create_test

import (
	"testing"

	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-test/pkg/cmd/create"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreate(t *testing.T) {
	_, o := create.NewCmdCreate()

	ns := "jx"
	o.TestGitURL = "https://github.com/myorg/env-pr-1243-5-bdd-thingy-dev.git"
	o.Namespace = ns
	o.TestClient = fake.NewSimpleClientset()
	o.Env = []string{
		"BRANCH_NAME=PR-1234",
		"BUILD_NUMBER=3",
		"PIPELINE_CONTEXT=gke-terraform-vault",
		"SOURCE_URL=https://github.com/jenkins-x/jx3-versions",
	}

	err := o.Run()
	require.NoError(t, err, "failed to run create command")

	// lets query the test...

	tests, err := o.TestClient.JxtestV1alpha1().TestRuns(ns).List(metav1.ListOptions{})
	require.NoError(t, err, "failed to list Tests in namespace %s", ns)
	require.Len(t, tests.Items, 1, "number of tests returned")

	test := tests.Items[0]

	t.Logf("found test %s\n", test.Name)
	t.Logf("found test spec %#v\n", test.Spec)

	// lets verify things are populated right
	assert.NotEmpty(t, test.Spec.Branch, "test.Spec.Branch")
	assert.NotEmpty(t, test.Spec.Context, "test.Spec.Context")
	assert.NotEmpty(t, test.Spec.BuildNumber, "test.Spec.BuildNumber")
	assert.NotEmpty(t, test.Spec.TriggerSource.URL, "test.Spec.TriggerSource.URL")
}
