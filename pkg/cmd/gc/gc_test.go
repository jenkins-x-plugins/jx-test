package gc_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner/fakerunner"
	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-test/pkg/cmd/create"
	"github.com/jenkins-x/jx-test/pkg/cmd/gc"
	"github.com/jenkins-x/jx-test/pkg/testruntesters"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testNamespace = "jx"
)

func TestShouldDeleteMarkedTestRun(t *testing.T) {
	t.Parallel()
	o, runner := CreateTestOptions()
	testRun := CreateTestRun(t, o, "168", "gke-terraform-vault", "1")
	testRun2 := CreateTestRun(t, o, "170", "gke-terraform-vault", "2")
	AssertDelete(t, o, testRun, false)
	AssertDelete(t, o, testRun2, false)

	testRun2.Spec.Delete = true
	AssertUpdate(t, o, testRun2)
	AssertDelete(t, o, testRun2, true)

	cmds := runner.OrderedCommands
	for _, c := range cmds {
		t.Logf("invoked command %s\n", c.CLI())
	}
	require.Len(t, cmds, 2, "command results")

	cloneCLI := cmds[0].CLI()
	prefix := "git clone https://github.com/myorg/env-pr-170-2-bdd-gke-terraform-vault-dev.git"
	assert.True(t, strings.HasPrefix(cloneCLI, prefix), "should have performed a git clone with command %s but was %s", prefix, cloneCLI)

	assert.Equal(t, "bin/destroy.sh", cmds[1].CLI())
}

func TestShouldDeleteNewerRunUsingNumericBuildNuymber(t *testing.T) {
	t.Parallel()
	o, _ := CreateTestOptions()

	testRun1 := CreateTestRun(t, o, "168", "gke-terraform-vault", "2")
	testRun2 := CreateTestRun(t, o, "55", "gke-terraform-vault", "2")
	testRun3 := CreateTestRun(t, o, "168", "gke-terraform-vault", "10")

	AssertDelete(t, o, testRun1, true)
	AssertDelete(t, o, testRun2, false)
	AssertDelete(t, o, testRun3, false)
}

func TestDeleteDueToNewerRun(t *testing.T) {
	t.Parallel()
	o, _ := CreateTestOptions()
	testRun1 := CreateTestRun(t, o, "168", "gke-terraform-vault", "159")
	testRun2 := CreateTestRun(t, o, "168", "gke-terraform-vault", "160")
	testRuns := make([]v1alpha1.TestRun, 0)
	testRuns = append(testRuns, *testRun1, *testRun2)
	assert.Equal(t, true, o.ShouldDeleteDueToNewerRun(testRun1, testRuns))
	testRun4 := CreateTestRun(t, o, "168", "gke-terraform-vault", "161")
	testRuns = append(testRuns, *testRun4)
	assert.Equal(t, false, o.ShouldDeleteDueToNewerRun(testRun4, testRuns))
}

func TestShouldDeleteOlderThanDuration(t *testing.T) {
	t.Parallel()
	o, _ := CreateTestOptions()
	testRun := CreateTestRun(t, o, "168", "gke-terraform-vault", "159")
	testRun.CreationTimestamp.Time = time.Now()
	assert.Equal(t, false, o.ShouldDeleteOlderThanDuration(testRun))
	testRun2 := CreateTestRun(t, o, "170", "gke-terraform-vault", "159")
	testRun2.CreationTimestamp.Time = time.Now().Add(-3 * time.Hour)
	assert.Equal(t, true, o.ShouldDeleteOlderThanDuration(testRun2))
	testRun2.Spec.Keep = true
	assert.Equal(t, false, o.ShouldDeleteOlderThanDuration(testRun2))
}

// CreateTestOptions creates an option for tests with a fake client
func CreateTestOptions() (*gc.Options, *fakerunner.FakeRunner) {
	_, o := gc.NewCmdGC()
	o.TestClient = fake.NewSimpleClientset()
	o.Namespace = testNamespace
	runner := &fakerunner.FakeRunner{}
	o.CommandRunner = runner.Run
	return o, runner
}

// AssertUpdate asserts we can update the given test run
func AssertUpdate(t *testing.T, o *gc.Options, tr *v1alpha1.TestRun) {
	testruntesters.RequireTestRunUpdate(t, o.TestClient, o.Namespace, tr)
}

func AssertDelete(t *testing.T, o *gc.Options, tr *v1alpha1.TestRun, deleted bool) {
	err := o.Run()
	require.NoError(t, err, "failed to run gc")
	testList, err := o.TestClient.JxtestV1alpha1().TestRuns(o.Namespace).List(metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		err = nil
	}
	require.NoError(t, err, "failed to find TestRuns in namespace %s", o.Namespace)
	for i := 0; i < len(testList.Items); i++ {
		r := testList.Items[i]
		if r.Name == tr.Name {
			if deleted {
				require.Fail(t, "should have deleted test run %s", tr.Name)
				return
			}
			t.Logf("as expected did not delete TestRun %s", tr.Name)
			return
		}
	}
	if !deleted {
		require.Fail(t, "should not have deleted TestRun %s", tr.Name)
	}
}

func CreateTestRun(t *testing.T, gco *gc.Options, prNumber, context, buildNumber string) *v1alpha1.TestRun {
	_, o := create.NewCmdCreate()
	o.TestClient = gco.TestClient
	o.Namespace = gco.Namespace
	o.TestGitURL = fmt.Sprintf("https://github.com/myorg/env-pr-%s-%s-bdd-%s-dev.git", prNumber, buildNumber, context)
	o.Env = []string{
		"BRANCH_NAME=PR-" + prNumber,
		"BUILD_NUMBER=" + buildNumber,
		"PIPELINE_CONTEXT=" + context,
		"REPO_URL=https://github.com/jenkins-x/jx3-versions",
	}
	err := o.Run()
	require.NoError(t, err, "failed to create TestRun for PR %s context %s buildNumber", prNumber, context, buildNumber)

	tr := o.TestRun
	require.NotNil(t, tr, "no TestRun created")

	// when using a fake provider the creation time stamp does not get populated
	if tr.CreationTimestamp.IsZero() {
		tr.CreationTimestamp.Time = time.Now()
		AssertUpdate(t, gco, tr)
	}
	return tr
}
