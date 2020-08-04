package gc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/alecthomas/assert"
	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-test/pkg/cmd/create"
	"github.com/jenkins-x/jx-test/pkg/cmd/gc"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testNamespace = "jx"
)

func TestDeleteDueToNewerRun(t *testing.T) {
	t.Parallel()
	o := CreateTestOptions()
	cluster1 := getCluster(t, o, "168", "gke-terraform-vault", "159")
	cluster2 := getCluster(t, o, "168", "gke-terraform-vault", "160")
	clusters := make([]v1alpha1.TestRun, 0)
	clusters = append(clusters, *cluster1, *cluster2)
	assert.Equal(t, true, o.ShouldDeleteDueToNewerRun(cluster1, clusters))
	cluster4 := getCluster(t, o, "168", "gke-terraform-vault", "161")
	clusters = append(clusters, *cluster4)
	assert.Equal(t, false, o.ShouldDeleteDueToNewerRun(cluster4, clusters))
}

func TestShouldDeleteMarkedCluster(t *testing.T) {
	t.Parallel()
	o := CreateTestOptions()
	cluster := getCluster(t, o, "168", "gke-terraform-vault", "159")
	cluster2 := getCluster(t, o, "170", "gke-terraform-vault", "159")
	AssertDelete(t, o, cluster, false)
	AssertDelete(t, o, cluster2, false)
	//assert.Equal(t, false, o.ShouldDeleteMarkedCluster(cluster))
	
	cluster2.Spec.Delete = true
	AssertUpdate(t, o, cluster2)
	AssertDelete(t, o, cluster2, true)
	//assert.Equal(t, true, o.ShouldDeleteMarkedCluster(cluster2))
}

func TestShouldDeleteOlderThanDuration(t *testing.T) {
	t.Parallel()
	o := CreateTestOptions()
	cluster := getCluster(t, o, "168", "gke-terraform-vault", "159")
	cluster.CreationTimestamp.Time = time.Now()
	assert.Equal(t, false, o.ShouldDeleteOlderThanDuration(cluster))
	cluster2 := getCluster(t, o, "170", "gke-terraform-vault", "159")
	cluster2.CreationTimestamp.Time = time.Now().Add(-3 * time.Hour)
	assert.Equal(t, true, o.ShouldDeleteOlderThanDuration(cluster2))
	cluster2.Spec.Keep = true
	assert.Equal(t, false, o.ShouldDeleteOlderThanDuration(cluster2))
}

// CreateTestOptions creates an option for tests with a fake client
func CreateTestOptions() *gc.Options {
	_, o := gc.NewCmdGC()
	o.TestClient = fake.NewSimpleClientset()
	o.Namespace = testNamespace
	return o
}

// AssertUpdate asserts we can update the given test run
func AssertUpdate(t *testing.T, o *gc.Options, tr *v1alpha1.TestRun) {
	_, err := o.TestClient.JxtestV1alpha1().TestRuns(o.Namespace).Update(tr)
	require.NoError(t, err, "failed to update TestRun %s", tr.Name)
}

func AssertDelete(t *testing.T, o *gc.Options, tr *v1alpha1.TestRun, deleted bool) {
	err := o.Run()
	require.NoError(t, err, "failed to run gc")
	testList, err := o.TestClient.JxtestV1alpha1().TestRuns(o.Namespace).List(metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		err = nil
	}
	require.NoError(t, err, "failed to find TestRuns in namespace %s", o.Namespace)
	for _, r := range testList.Items {
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

func getCluster(t *testing.T, gco *gc.Options, prNumber string, context string, buildNumber string) *v1alpha1.TestRun {
	_, o := create.NewCmdCreate()
	o.TestClient = gco.TestClient
	o.Namespace = gco.Namespace
	o.TestGitURL = fmt.Sprintf("https://github.com/myorg/env-pr-%s-%s-bdd-%s-dev.git", prNumber, buildNumber, context)
	o.Env = []string{
		"BRANCH_NAME=PR-" + prNumber,
		"BUILD_NUMBER=" + buildNumber,
		"PIPELINE_CONTEXT=" + context,
		"SOURCE_URL=https://github.com/jenkins-x/jx3-versions",
	}
	err := o.Run()
	require.NoError(t, err, "failed to create TestRun for PR %s context %s buildNumber", prNumber, context, buildNumber)

	tr := o.TestRun
	require.NotNil(t, tr, "no TestRun created")
	return tr
}
