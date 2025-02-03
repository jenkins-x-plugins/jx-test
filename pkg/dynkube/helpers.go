package dynkube

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// DynamicResource creates the client interface
func DynamicResource(dynamicClient dynamic.Interface, ns string, gvr schema.GroupVersionResource) dynamic.ResourceInterface {
	var client dynamic.ResourceInterface
	if ns != "" {
		client = dynamicClient.Resource(gvr).Namespace(ns)
	} else {
		client = dynamicClient.Resource(gvr)
	}
	return client
}

// ToSelector converts the given labels into a selector string
func ToSelector(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	buf := &strings.Builder{}
	for k, v := range labels {
		if buf.Len() > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(v)
	}
	return buf.String()
}
