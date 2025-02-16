package gc

import (
	"context"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v69/github"
	"net/http"
	"strings"
	"time"

	"github.com/jenkins-x-plugins/jx-test/pkg/dynkube"
	"github.com/jenkins-x-plugins/jx-test/pkg/terraforms"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"k8s.io/client-go/kubernetes"

	"github.com/jenkins-x-plugins/jx-test/pkg/root"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

var (
	info = termcolor.ColorInfo

	cmdLong = templates.LongDesc(`
		Garbage collects test resources
`)

	cmdExample = templates.Examples(`
		%s gc
	`)

	terraformStateSelector = "tfstate=true"

	defaultTerraformConfigMapPrefix = "tf-jx3-versions-"
)

// Options the options for the command
type Options struct {
	Selector                 string
	Namespace                string
	TerraformConfigMapPrefix string
	Duration                 time.Duration
	KubeClient               kubernetes.Interface
	DynamicClient            dynamic.Interface
	Ctx                      context.Context
	Client                   dynamic.ResourceInterface
	CommandRunner            cmdrunner.CommandRunner
	AppID                    int64
	AppCertificateFile       string
}

// NewCmdGC creates a command object for the command
func NewCmdGC() (*cobra.Command, *Options) {
	o := &Options{}

	cmd := &cobra.Command{
		Use:     "gc",
		Short:   "Garbage collects test resources",
		Long:    cmdLong,
		Example: fmt.Sprintf(cmdExample, root.BinaryName),
		Run: func(_ *cobra.Command, _ []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	if o.Ctx == nil {
		o.Ctx = cmd.Context()
	}

	cmd.Flags().StringVarP(&o.Namespace, "ns", "n", "", "the namespace to query the Terraform resources")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", "kind="+terraforms.LabelValueKindTest, "the selector to find the Terraform resources to remove")
	cmd.Flags().StringVarP(&o.TerraformConfigMapPrefix, "tf-cm-prefix", "t", defaultTerraformConfigMapPrefix, "the ConfigMap name prefix of the Terraform state")
	cmd.Flags().DurationVarP(&o.Duration, "duration", "d", 2*time.Hour, "The maximum age of a Terraform resource before it is garbage collected")
	cmd.Flags().Int64Var(&o.AppID, "app-id", 0, "GitHub App ID used to gc repositories")
	cmd.Flags().StringVar(&o.AppCertificateFile, "app-certificate-file", "", "Certificate for GitHub App used to gc repositories")
	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return fmt.Errorf("failed to validate setup: %w", err)
	}

	ctx := o.GetContext()
	ns := o.Namespace
	gvr := terraforms.TerraformResource
	o.Client = dynkube.DynamicResource(o.DynamicClient, ns, gvr)

	kind := "Terraform"

	// lets delete all the previous resources for this Pull Request and Context
	list, err := o.Client.List(ctx, metav1.ListOptions{
		LabelSelector: o.Selector,
	})
	if err != nil && apierrors.IsNotFound(err) {
		return fmt.Errorf("could not find resources for : %w", err)
	}

	createdBefore := time.Now().Add(o.Duration * -1)
	createdTime := &metav1.Time{
		Time: createdBefore,
	}
	for _, r := range list.Items {
		name := r.GetName()

		labels := r.GetLabels()
		if labels != nil {
			keep := labels["keep"]
			if keep != "" {
				log.Logger().Infof("not removing %s %s as it has a keep label", kind, info(name))
				continue
			}
		}

		created := r.GetCreationTimestamp()
		if !created.Before(createdTime) {
			log.Logger().Infof("not removing %s %s as it was created at %s", kind, info(name), created.String())
			continue
		}

		err = o.deleteTerraform(ctx, kind, name)
		if err != nil {
			return fmt.Errorf("failed to delete %s %s: %w", kind, name, err)
		}

		log.Logger().Infof("deleted %s %s since it was created at: %s", kind, info(name), created.String())
	}

	err = o.gcLeases(ctx, createdTime)
	if err != nil {
		return fmt.Errorf("failed to GC leases: %w", err)
	}

	err = o.gcTerraformState(ctx, createdTime)
	if err != nil {
		return fmt.Errorf("failed to GC terraform state: %w", err)
	}

	err = o.gcTerraformConfigMaps(ctx, createdTime)
	if err != nil {
		return fmt.Errorf("failed to GC terraform configs: %w", err)
	}
	err = o.gcRepositories(ctx, createdTime)
	if err != nil {
		return fmt.Errorf("failed to GC test repsitories: %w", err)
	}
	return nil
}

func (o *Options) deleteTerraform(ctx context.Context, kind, name string) error {
	ns := o.Namespace
	err := terraforms.DeleteActiveTerraformJobs(ctx, o.KubeClient, ns, name)
	if err != nil {
		return fmt.Errorf("failed to delete active Terraform Jobs for namespace %s name %s: %w", ns, name, err)
	}

	log.Logger().Infof("deleting %s %s", kind, info(name))

	c := &cmdrunner.Command{
		Name: "kubectl",
		Args: []string{"patch", kind, name, "-p", "{\"metadata\": {\"finalizers\": []}}", "--type=merge"},
	}
	_, err = o.CommandRunner(c)
	if err != nil {
		return fmt.Errorf("failed to run %s: %w", c.CLI(), err)
	}
	c = &cmdrunner.Command{
		Name: "kubectl",
		Args: []string{"delete", kind, name},
	}
	_, err = o.CommandRunner(c)
	if err != nil {
		return fmt.Errorf("failed to run %s: %w", c.CLI(), err)
	}
	return nil
}

func (o *Options) Validate() error {
	if o.CommandRunner == nil {
		o.CommandRunner = cmdrunner.QuietCommandRunner
	}
	var err error
	o.KubeClient, o.Namespace, err = kube.LazyCreateKubeClientAndNamespace(o.KubeClient, o.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create kube client: %w", err)
	}
	o.DynamicClient, err = kube.LazyCreateDynamicClient(o.DynamicClient)
	if err != nil {
		return fmt.Errorf("failed to craete dynamic client: %w", err)
	}
	return nil
}

// GetContext lazily creates a context if it doesn't exist already
func (o *Options) GetContext() context.Context {
	if o.Ctx == nil {
		o.Ctx = context.TODO()
	}
	return o.Ctx
}

func (o *Options) gcLeases(ctx context.Context, createdTime *metav1.Time) error {
	leaseInterface := o.KubeClient.CoordinationV1().Leases(o.Namespace)
	list, err := leaseInterface.List(ctx, metav1.ListOptions{
		LabelSelector: terraformStateSelector,
	})
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("failed to list Leases in namespace %s with selector %s: %w", o.Namespace, terraformStateSelector, err)
	}
	if list == nil {
		return nil
	}

	for _, r := range list.Items {
		created := r.GetCreationTimestamp()
		if !created.Before(createdTime) {
			log.Logger().Debugf("not removing Lease %s as it was created at %s", r.Name, created.String())
			continue
		}
		err = leaseInterface.Delete(ctx, r.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete Lease %s in namespace %s: %w", r.Name, o.Namespace, err)
		}
		log.Logger().Infof("deleted Lease %s", r.Name)
	}
	return nil
}

func (o *Options) gcTerraformState(ctx context.Context, createdTime *metav1.Time) error {
	secretInterface := o.KubeClient.CoreV1().Secrets(o.Namespace)

	list, err := secretInterface.List(ctx, metav1.ListOptions{
		LabelSelector: terraformStateSelector,
	})
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("failed to list Secrets in namespace %s with selector %s: %w", o.Namespace, terraformStateSelector, err)
	}
	if list == nil {
		return nil
	}

	for _, r := range list.Items {
		created := r.GetCreationTimestamp()
		if !created.Before(createdTime) {
			log.Logger().Debugf("not removing Secret %s as it was created at %s", r.Name, created.String())
			continue
		}
		err = secretInterface.Delete(ctx, r.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete Secret %s in namespace %s: %w", r.Name, o.Namespace, err)
		}
		log.Logger().Infof("deleted Secret %s", r.Name)
	}
	return nil
}

func (o *Options) gcTerraformConfigMaps(ctx context.Context, createdTime *metav1.Time) error {
	if o.TerraformConfigMapPrefix == "" {
		o.TerraformConfigMapPrefix = defaultTerraformConfigMapPrefix
	}

	configMapInterface := o.KubeClient.CoreV1().ConfigMaps(o.Namespace)

	list, err := configMapInterface.List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("failed to list ConfigMaps in namespace %s with selector %s: %w", o.Namespace, terraformStateSelector, err)
	}
	if list == nil {
		return nil
	}

	for _, r := range list.Items {
		if !strings.HasPrefix(r.Name, o.TerraformConfigMapPrefix) {
			continue
		}
		created := r.GetCreationTimestamp()
		if !created.Before(createdTime) {
			log.Logger().Debugf("not removing ConfigMap %s as it was created at %s", r.Name, created.String())
			continue
		}
		err = configMapInterface.Delete(ctx, r.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete ConfigMap %s in namespace %s: %w", r.Name, o.Namespace, err)
		}
		log.Logger().Infof("deleted ConfigMap %s", r.Name)
	}
	return nil
}

func (o *Options) gcRepositories(ctx context.Context, createdTime *metav1.Time) error {
	if o.AppID == 0 || o.AppCertificateFile == "" {
		log.Logger().Infof("--app-id and --app-certificate-file are not specified, so no repositories are garbage collected")
		return nil
	}
	log.Logger().Infof("cleaning repositories")
	itr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, o.AppID, o.AppCertificateFile)
	if err != nil {
		log.Logger().Fatalf("failed to configure transport as app (%d): %v", o.AppID, err)
	}
	client := github.NewClient(&http.Client{Transport: itr})

	installations, _, err := client.Apps.ListInstallations(context.Background(), &github.ListOptions{})
	if err != nil {
		log.Logger().Fatalf("failed to list installations: %v\n", err)
	}

	// capture our installationId for our app
	// we need this for the access token
	var installID int64
	for _, val := range installations {
		installID = val.GetID()
	}
	log.Logger().Infof("found installation %d", installID)

	token, _, err := client.Apps.CreateInstallationToken(
		context.Background(),
		installID,
		&github.InstallationTokenOptions{})
	if err != nil {
		return fmt.Errorf("failed to create installation token: %v\n", err)
	}

	apiClient := github.NewClient(nil).WithAuthToken(
		token.GetToken(),
	)
	repos, _, err := apiClient.Repositories.ListByOrg(ctx, "jenkins-x-bdd", &github.RepositoryListByOrgOptions{})
	if err != nil {
		return err
	}
	log.Logger().Infof("got %d repos", len(repos))
	for i := range repos {
		repo := repos[i]
		log.Logger().Infof("maybe removing repository %s", repo.Name)
		if !repo.CreatedAt.Before(createdTime.Time) {
			log.Logger().Infof("not removing repository %s as it was created at %s", repo.Name, repo.CreatedAt.String())
			continue
		}
		_, err = apiClient.Repositories.Delete(ctx,
			"jenkins-x-bdd",
			*repo.Name)
		if err != nil {
			return fmt.Errorf("failed to delete the repository %s/%s: %w", *repo.Owner.Name, *repo.Name, err)
		}

	}
	return nil
}
