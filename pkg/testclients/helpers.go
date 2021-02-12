package testclients

import (
	"github.com/jenkins-x/jx-kube-client/v3/pkg/kubeclient"
	"github.com/jenkins-x/jx-test/pkg/apis/jxtest/v1alpha1"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LazyCreateClient lazy creates the test client if its not defined
func LazyCreateClient(client versioned.Interface, ns string) (versioned.Interface, string, error) {
	if ns == "" {
		var err error
		ns, err = kubeclient.CurrentNamespace()
		if err != nil {
			return nil, ns, errors.Wrapf(err, "failed to find current namespace")
		}

	}
	if client != nil {
		return client, ns, nil
	}
	f := kubeclient.NewFactory()
	cfg, err := f.CreateKubeConfig()
	if err != nil {
		return client, ns, errors.Wrap(err, "failed to get kubernetes config")
	}
	client, err = versioned.NewForConfig(cfg)
	if err != nil {
		return client, ns, errors.Wrap(err, "error building tests clientset")
	}
	return client, ns, nil
}

// ListTestRuns loads the test clients from the given namespace
func ListTestRuns(testClient versioned.Interface, ns string) ([]v1alpha1.TestRun, error) {
	testList, err := testClient.JxtestV1alpha1().TestRuns(ns).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, errors.Wrapf(err, "failed to list TestRun instances in namespace %s", ns)
	}
	return testList.Items, nil
}

// MarkDeleted marks the test run as being deleted
func MarkDeleted(testClient versioned.Interface, ns string, testRun *v1alpha1.TestRun) error {
	if testRun.Spec.Delete {
		return nil
	}
	testRun.Spec.Delete = true

	_, err := testClient.JxtestV1alpha1().TestRuns(ns).Update(testRun)
	if err != nil {
		return errors.Wrapf(err, "failed to update TestRun %s in namespace %s", testRun.Name, ns)
	}
	return nil
}
