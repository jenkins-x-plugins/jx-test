package terraforms

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (

	// LabelValueKindTest the kind label value for tests
	LabelValueKindTest = "jx-test"
)

var (
	// TerraformResource the Terraform Operator resource
	// see:
	TerraformResource = schema.GroupVersionResource{Group: "tf.isaaguilar.com", Version: "v1alpha1", Resource: "terraforms"}
)
