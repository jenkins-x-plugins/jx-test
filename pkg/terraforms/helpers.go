package terraforms

import (
	"context"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jobs"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	info = termcolor.ColorInfo
)

// DeleteActiveTerraformJobs deletes any non completed apply Terraform Jobs as we are about to remove the
// Terraform resource
func DeleteActiveTerraformJobs(ctx context.Context, kubeClient kubernetes.Interface, ns, name string) error {
	jobInterface := kubeClient.BatchV1().Jobs(ns)
	job, err := jobInterface.Get(ctx, name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to query Job %s in namespace %s", name, ns)
	}
	if job == nil || jobs.IsJobFinished(job) {
		return nil
	}
	log.Logger().Infof("deleting terraform apply Job %s in namespace %s as has not finished and we are about to delete the Terraform resource", info(name), ns)
	err = jobInterface.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to delete Job %s in namespace %s", name, ns)
	}
	log.Logger().Infof("deleted terraform apply Job %s in namespace %s", info(name), ns)
	return nil
}
