package testruntesters

import (
	"testing"

	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RequireTestRunUpdate asserts that we can update the given test run
func RequireTestRunUpdate(t *testing.T, testClient versioned.Interface, ns string, tr *v1alpha1.TestRun) {
	_, err := testClient.JxtestV1alpha1().TestRuns(ns).Update(tr)
	require.NoError(t, err, "failed to update TestRun %s", tr.Name)
}

// RequireModifyTestRun asserts that the test run with the given test URL can be modified correctly
func RequireModifyTestRun(t *testing.T, testClient versioned.Interface, ns, testGitURL string, fn func(tr *v1alpha1.TestRun) error) {
	testList, err := testClient.JxtestV1alpha1().TestRuns(ns).List(metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		err = nil
	}
	require.NoError(t, err, "failed to list TestRun instances in namespace %s", ns)

	for i := 0; i < len(testList.Items); i++ {
		tr := &testList.Items[i]
		if tr.Spec.TestSource.URL == testGitURL {
			err = fn(tr)
			require.NoError(t, err, "failed to modify TestRun %s in namespace %s", tr.Name, ns)

			RequireTestRunUpdate(t, testClient, ns, tr)
			return
		}
	}
	require.Fail(t, "did not find TestRun in namespace %s for test Git URL %s", ns, testGitURL)
}
