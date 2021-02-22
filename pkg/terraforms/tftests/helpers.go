package tftests

import (
	"github.com/jenkins-x-plugins/jx-test/pkg/terraforms"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
	"testing"

	"github.com/stretchr/testify/require"
	dynfake "k8s.io/client-go/dynamic/fake"
)

// ParseUnstructureds parses the resources
func ParseUnstructureds(t *testing.T, fn func(idx int, u *unstructured.Unstructured), resources []string) []runtime.Object {
	var answer []runtime.Object
	for i, r := range resources {
		u := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(r), u)
		require.NoError(t, err, "failed to unmarshal resource %s", r)

		if fn != nil {
			fn(i, u)
		}
		answer = append(answer, u)
	}
	return answer
}

// NewFakeDynClient creates a new dynamic client with the external secrets
func NewFakeDynClient(scheme *runtime.Scheme, dynObjects ...runtime.Object) *dynfake.FakeDynamicClient {
	gvrToListKind := map[schema.GroupVersionResource]string{
		terraforms.TerraformResource: "TerraformList",
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, dynObjects...)
}
