package create_test

import (
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner/fakerunner"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strconv"
	"testing"

	"github.com/jenkins-x/jx-test/pkg/cmd/create"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dynfake "k8s.io/client-go/dynamic/fake"
)

var (
	testResources = []string{
		`apiVersion: tf.isaaguilar.com/v1alpha1
kind: Terraform
metadata:
  labels:
    context: myctx
    owner: myowner
    pr: pr-456
    repo: myrepo
  name: tf-myrepo-pr456-myctx-1
  namespace: jx
`,
		`apiVersion: tf.isaaguilar.com/v1alpha1
kind: Terraform
metadata:
  labels:
    context: myctx
    owner: myowner
    pr: pr-456
    repo: myrepo
  name: tf-myrepo-pr456-myctx-2
  namespace: jx
`,
		`apiVersion: tf.isaaguilar.com/v1alpha1
kind: Terraform
metadata:
  labels:
    context: myctx
    owner: myowner
    pr: pr-999
    repo: myrepo
  name: tf-myrepo-pr999-myctx-3
  namespace: jx
`,
	}
)

func TestCreate(t *testing.T) {
	ns := "jx"
	namePrefix := "tf-"
	owner := "myowner"
	repo := "myrepo"
	contextName := "myctx"
	prNumber := 456
	buildNumber := "3"
	prLabel := "pr-" + strconv.Itoa(prNumber)
	expectedName := "tf-myrepo-pr456-myctx-3"

	scheme := runtime.NewScheme()
	dynObjects := ParseUnstructureds(t, testResources)
	fakeDynClient := NewFakeDynClient(scheme, dynObjects...)

	runner := &fakerunner.FakeRunner{}

	_, o := create.NewCmdCreate()
	o.PullRequestNumber = prNumber
	o.RepoOwner = owner
	o.RepoName = repo
	o.Context = contextName
	o.BuildNumber = buildNumber
	o.Namespace = ns
	o.ResourceNamePrefix = namePrefix
	o.EnvVars = []string{"TF_VAR_gcp_project=jenkins-x-labs-bdd", "TF_VAR_cluster_name=pr-2127-5-gke-gsm"}
	o.File = filepath.Join("test_data", "tf.yaml")
	o.DynamicClient = fakeDynClient
	o.CommandRunner = runner.Run

	err := o.Run()
	require.NoError(t, err, "failed to run create command")

	assert.Equal(t, expectedName, o.ResourceName, "o.ResourceName")
	assert.Equal(t, map[string]string{"context": contextName, "owner": owner, "pr": prLabel, "repo": repo}, o.Labels, "o.Labels")

	ctx := o.GetContext()

	list, err := o.Client.List(ctx, metav1.ListOptions{})
	require.NoError(t, err, "failed to list resources")
	require.NotNil(t, list, "no list resource returned")
	require.Len(t, list.Items, 2, "should have two resources after removing the previous PRs resources")

	r := list.Items[0]
	require.Equal(t, expectedName, r.GetName(), "resource[0].Name")
	require.Equal(t, ns, r.GetNamespace(), "resource[0].Namespace")

	data, err := yaml.Marshal(r)
	require.NoError(t, err, "failed to marshal resource %v", r)
	t.Logf("got resource %s\n", string(data))

	r = list.Items[1]
	require.Equal(t, "tf-myrepo-pr999-myctx-3", r.GetName(), "resource[1].Name")
	require.Equal(t, ns, r.GetNamespace(), "resource[1].Namespace")

	for _, c := range runner.OrderedCommands {
		t.Logf("faked: %s\n", c.CLI())
	}
}

// ParseUnstructureds parses the resources
func ParseUnstructureds(t *testing.T, resources []string) []runtime.Object {
	var answer []runtime.Object
	for _, r := range resources {
		u := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(r), u)
		require.NoError(t, err, "failed to unmarshal resource %s", r)
		answer = append(answer, u)
	}
	return answer
}

// NewFakeDynClient creates a new dynamic client with the external secrets
func NewFakeDynClient(scheme *runtime.Scheme, dynObjects ...runtime.Object) *dynfake.FakeDynamicClient {
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "tf.isaaguilar.com", Version: "v1alpha1", Resource: "terraforms"}: "TerraformList",
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, dynObjects...)
}
