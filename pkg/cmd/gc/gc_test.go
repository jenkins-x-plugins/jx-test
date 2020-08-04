package gc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/alecthomas/assert"
	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-test/pkg/cmd/gc"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleteDueToNewerRun(t *testing.T) {
	t.Parallel()
	o := CreateTestOptions()
	cluster1 := getCluster(t, "168", "gke-terraform-vault", "159")
	cluster2 := getCluster(t, "168", "gke-terraform-vault", "160")
	clusters := make([]v1alpha1.TestRun, 0)
	clusters = append(clusters, *cluster1, *cluster2)
	assert.Equal(t, true, o.ShouldDeleteDueToNewerRun(cluster1, clusters))
	cluster4 := getCluster(t, "168", "gke-terraform-vault", "160")
	clusters = append(clusters, *cluster4)
	assert.Equal(t, false, o.ShouldDeleteDueToNewerRun(cluster4, clusters))
}

// TODO
/*
func TestShouldDeleteMarkedCluster(t *testing.T) {
	t.Parallel()
	o := CreateTestOptions()
	cluster := getCluster(t, "168", "gke-terraform-vault", "159")
	cluster2 := getCluster(t, "170", "gke-terraform-vault", "159")
	assert.Equal(t, false, o.ShouldDeleteMarkedCluster(cluster))
	cluster2.Spec.Delete = true
	assert.Equal(t, true, o.ShouldDeleteMarkedCluster(cluster2))
}

 */


func TestShouldDeleteOlderThanDuration(t *testing.T) {
	t.Parallel()
	o := CreateTestOptions()
	cluster := getCluster(t, "168", "gke-terraform-vault", "159")
	cluster.CreationTimestamp.Time = time.Now()
	assert.Equal(t, false, o.ShouldDeleteOlderThanDuration(cluster))
	cluster2 := getCluster(t, "170", "gke-terraform-vault", "159")
	cluster2.CreationTimestamp.Time = time.Now().Add(-3 * time.Hour)
	assert.Equal(t, true, o.ShouldDeleteOlderThanDuration(cluster2))
	cluster2.Spec.Keep = true
	assert.Equal(t, false, o.ShouldDeleteOlderThanDuration(cluster2))
}

// CreateTestOptions creates an option for tests with a fake client
func CreateTestOptions() *gc.Options {
	_, o := gc.NewCmdGC()
	o.TestClient = fake.NewSimpleClientset()
	return o
}

func getCluster(t *testing.T, prNumber string, context string, buildNumber string) *v1alpha1.TestRun {
	tr := &v1alpha1.TestRun{
		TypeMeta:   v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{},
		Spec:       v1alpha1.TestRunSpec{
			TestSource:    v1alpha1.Source{
				URL: fmt.Sprintf("https://github.com/myorg/env-pr-%s-%s-bdd-%s-dev.git", prNumber, buildNumber, context),
			},
			RemoveScript:  "bin/destroy.sh",
			TriggerSource: v1alpha1.Source{},
			Env: map[string]string{
				"BRANCH_NAME" : "PR-" + prNumber,
				"BUILD_NUMBER" : buildNumber,
				"PIPELINE_CONTEXT" : context,
				"SOURCE_URL" : "https://github.com/jenkins-x/jx3-versions",
			},
		},
	}
	err := tr.Spec.Validate()
	require.NoError(t, err, "failed to create TestRun for PR %s context %s buildNumber", prNumber, context, buildNumber)
	return tr
}
