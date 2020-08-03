package testclients

import (
	"github.com/jenkins-x/jx-kube-client/pkg/kubeclient"
	"github.com/jenkins-x/jx-test/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
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
