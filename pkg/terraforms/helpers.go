package terraforms

import (
	"context"
	"fmt"

	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jobs"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"

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
		return deleteTerraformPods(ctx, kubeClient, ns, name)
	}
	if err != nil {
		return fmt.Errorf("failed to query Job %s in namespace %s: %w", name, ns, err)
	}
	if job == nil || jobs.IsJobFinished(job) {
		return deleteTerraformPods(ctx, kubeClient, ns, name)
	}
	log.Logger().Infof("deleting terraform apply Job %s in namespace %s as has not finished and we are about to delete the Terraform resource", info(name), ns)
	err = jobInterface.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete Job %s in namespace %s: %w", name, ns, err)
	}
	log.Logger().Infof("deleted terraform apply Job %s in namespace %s", info(name), ns)
	return deleteTerraformPods(ctx, kubeClient, ns, name)
}

func deleteTerraformPods(ctx context.Context, kubeClient kubernetes.Interface, ns, name string) error {
	selector := "job-name=" + name
	podInterface := kubeClient.CoreV1().Pods(ns)
	podList, err := podInterface.List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to query Pods namespace %s with selector %s: %w", ns, selector, err)
	}
	if podList == nil {
		return nil
	}

	for i := range podList.Items {
		name := podList.Items[i].Name
		err = podInterface.Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete pod %s: %w", name, err)
		}
		log.Logger().Infof("deleted terraform apply Pod %s in namespace %s", info(name), ns)
	}
	return nil
}
