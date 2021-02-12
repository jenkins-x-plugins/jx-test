package create_test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	fakeDynClient := NewFakeDynClient(scheme)

	_, o := create.NewCmdCreate()

	o.PullRequestNumber = prNumber
	o.RepoOwner = owner
	o.RepoName = repo
	o.Context = contextName
	o.BuildNumber = buildNumber
	o.Namespace = ns
	o.ResourceNamePrefix = namePrefix
	o.File = filepath.Join("test_data", "tf.yaml")
	o.DynamicClient = fakeDynClient

	err := o.Run()
	require.NoError(t, err, "failed to run create command")

	assert.Equal(t, expectedName, o.ResourceName, "o.ResourceName")
	assert.Equal(t, map[string]string{"context": contextName, "owner": owner, "pr": prLabel, "repo": repo}, o.Labels, "o.Labels")

	ctx := o.GetContext()

	list, err := o.Client.List(ctx, metav1.ListOptions{})
	require.NoError(t, err, "failed to list resources")
	require.NotNil(t, list, "no list resource returned")
	require.Len(t, list.Items, 1, "should have one resource")
	r := list.Items[0]
	require.Equal(t, expectedName, r.GetName(), "resource.Name")
	require.Equal(t, ns, r.GetNamespace(), "resource.Namespace")

	data, err := yaml.Marshal(r)
	require.NoError(t, err, "failed to marshal resource %v", r)
	t.Logf("got resource %s\n", string(data))
}

// NewFakeDynClient creates a new dynamic client with the external secrets
func NewFakeDynClient(scheme *runtime.Scheme, dynObjects ...runtime.Object) *dynfake.FakeDynamicClient {
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "tf.isaaguilar.com", Version: "v1alpha1", Resource: "terraforms"}: "TerraformList",
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, dynObjects...)
}
