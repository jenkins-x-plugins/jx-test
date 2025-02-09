package create

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jenkins-x-plugins/jx-test/pkg/dynkube"
	"github.com/jenkins-x-plugins/jx-test/pkg/terraforms"
	"k8s.io/client-go/kubernetes"

	"github.com/Masterminds/sprig/v3"
	"github.com/jenkins-x-plugins/jx-test/pkg/root"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-helpers/v3/pkg/pipelinectx"
	"github.com/jenkins-x/jx-helpers/v3/pkg/templater"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

var (
	info = termcolor.ColorInfo

	cmdLong = templates.LongDesc(`
		Garbage collects test resources
`)

	cmdExample = templates.Examples(`
		%s create --test-url https://github.com/myorg/mytest.git
	`)
)

// Options the options for the command
type Options struct {
	pipelinectx.Options
	File             string
	Name             string
	Namespace        string
	EnvPattern       string
	NoWatchJob       bool
	NoDeleteResource bool
	LogResource      bool
	VerifyResult     bool
	Env              map[string]string
	EnvVars          []string
	KubeClient       kubernetes.Interface
	DynamicClient    dynamic.Interface
	Ctx              context.Context
	Client           dynamic.ResourceInterface
	CommandRunner    cmdrunner.CommandRunner
}

// NewCmdCreate creates a command object for the command
func NewCmdCreate() (*cobra.Command, *Options) {
	o := &Options{}

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new TestRun resource to record the test case resources",
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
	err := o.EnvironmentDefaults(o.GetContext())
	if err != nil {
		log.Logger().Warnf("failed to default env vars: %s", err.Error())
	}
	if o.Options.ResourceNamePrefix == "" {
		o.Options.ResourceNamePrefix = "tf-"
	}

	o.Options.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.File, "file", "f", "", "the template file to create")
	cmd.Flags().StringVarP(&o.EnvPattern, "env-pattern", "", "TF_.*", "the regular expression for environment variables to automatically include")
	cmd.Flags().StringArrayVarP(&o.EnvVars, "env", "e", nil, "specifies env vars of the form name=value")
	cmd.Flags().BoolVarP(&o.NoWatchJob, "no-watch-job", "", false, "disables watching of the job created by the resource")
	cmd.Flags().BoolVarP(&o.NoDeleteResource, "no-delete", "", false, "disables deleting of the test resource after the job has completed successfully")
	cmd.Flags().BoolVarP(&o.LogResource, "log", "", true, "logs the generated resource before applying it")
	cmd.Flags().BoolVarP(&o.VerifyResult, "verify-result", "", false, "verifies the output of the boot job to ensure it succeeded")
	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return fmt.Errorf("failed to validate: %w", err)
	}

	log.Logger().Infof("resource: %s", info(o.ResourceName))
	log.Logger().Infof("labels: %v", o.Labels)

	templateText, err := os.ReadFile(o.File)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", o.File, err)
	}

	o.Name = o.ResourceName
	funcMap := sprig.TxtFuncMap()

	output, err := templater.Evaluate(funcMap, o, string(templateText), o.File, "resource template")
	if err != nil {
		return fmt.Errorf("failed to evaluate template %s: %w", o.File, err)
	}

	if o.LogResource {
		log.Logger().Infof("generated template: %s", output)
	}

	u := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(output), u)
	if err != nil {
		return fmt.Errorf("failed to unmarshal the template of file %s has YAML: %s: %w", o.File, output, err)
	}

	// modify labels
	labels := u.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	if o.Labels["kind"] == "" {
		o.Labels["kind"] = terraforms.LabelValueKindTest
	}
	for k, v := range o.Labels {
		labels[k] = v
	}
	u.SetLabels(labels)

	// modify name
	u.SetName(o.ResourceName)
	ns := o.Namespace
	if ns != "" {
		u.SetNamespace(ns)
	}

	kind := u.GetKind()
	apiVersion := u.GetAPIVersion()
	if kind == "" {
		return fmt.Errorf("generated template of file %s has missing kind", o.File)
	}
	if apiVersion == "" {
		return fmt.Errorf("generated template of file %s has missing apiVersion", o.File)
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return fmt.Errorf("failed to parse apiVersion: %s: %w", apiVersion, err)
	}
	resourceName := strings.ToLower(kind) + "s"
	gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: resourceName}

	o.Client = dynkube.DynamicResource(o.DynamicClient, ns, gvr)
	ctx := o.GetContext()
	selector := dynkube.ToSelector(o.Labels)

	// lets delete all the previous resources for this Pull Request and Context
	list, err := dynkube.DynamicResource(o.DynamicClient, ns, gvr).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil && apierrors.IsNotFound(err) {
		return fmt.Errorf("could not find resources for : %w", err)
	}
	if list != nil {
		for _, r := range list.Items {
			name := r.GetName()

			err = terraforms.DeleteActiveTerraformJobs(ctx, o.KubeClient, ns, name)
			if err != nil {
				return fmt.Errorf("failed to delete active Terraform Jobs for namespace %s name %s: %w", ns, name, err)
			}

			err = dynkube.DynamicResource(o.DynamicClient, ns, gvr).Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("failed to delete %s: %w", name, err)
			}
			log.Logger().Infof("deleted previous pipeline %s %s", kind, info(name))
		}
	}

	// now lets create the new resource
	name := o.Name
	if name == "" {
		return fmt.Errorf("no name defaulted")
	}

	_, err = dynkube.DynamicResource(o.DynamicClient, ns, gvr).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("should not have a %s called %s", kind, name)
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check if %s %s exists: %w", kind, name, err)
	}

	_, err = dynkube.DynamicResource(o.DynamicClient, ns, gvr).Create(ctx, u, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create %s %s: %w", kind, name, err)
	}
	log.Logger().Infof("created %s %s", kind, info(name))

	if o.NoWatchJob {
		return nil
	}
	err = o.watchJob()
	if err != nil {
		return fmt.Errorf("job failed to complete successfully: %w", err)
	}

	if o.NoDeleteResource {
		return nil
	}

	tf, err := dynkube.DynamicResource(o.DynamicClient, ns, gvr).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to find Terraform %s in namespace %s: %w", name, ns, err)
	}

	labels = tf.GetLabels()
	if labels != nil {
		keep := labels["keep"]
		log.Logger().Infof("test Terraform %s in namespace %s has keep label %s", info(name), info(ns), info(keep))
		if keep == "yes" || keep == "true" {
			log.Logger().Infof("not removing the test Terraform %s in namespace %s as it has a keep label", info(name), info(ns))
			return nil
		}
	}

	err = dynkube.DynamicResource(o.DynamicClient, ns, gvr).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete %s %s: %w", kind, name, err)
	}
	log.Logger().Infof("Job succeeded so deleted %s %s", kind, info(name))
	return nil
}

// Validate validates options
func (o *Options) Validate() error {
	err := o.Options.Validate()
	if err != nil {
		return fmt.Errorf("failed to validate pipeline options: %w", err)
	}
	if o.CommandRunner == nil {
		o.CommandRunner = cmdrunner.DefaultCommandRunner
	}

	if o.File == "" {
		return options.MissingOption("file")
	}
	exists, err := files.FileExists(o.File)
	if err != nil {
		return fmt.Errorf("failed to check if file exists %s: %w", o.File, err)
	}
	if !exists {
		return fmt.Errorf("file %s does not exist", o.File)
	}

	// lets delete any old resources
	if len(o.Labels) == 0 {
		return fmt.Errorf("no labels could be created")
	}

	if o.Env == nil {
		o.Env = map[string]string{}
	}
	for _, e := range o.EnvVars {
		values := strings.SplitN(e, "=", 2)
		if len(values) < 2 {
			return options.InvalidOptionf("env", e, "environment variables should be of the form name=value")
		}
		o.Env[values[0]] = values[1]
	}

	if o.EnvPattern != "" {
		r, err := regexp.Compile(o.EnvPattern)
		if err != nil {
			return fmt.Errorf("failed to parse option --env-pattern %s: %w", o.EnvPattern, err)
		}

		// lets include any environment variables too
		for _, e := range os.Environ() {
			values := strings.SplitN(e, "=", 2)
			if len(values) < 2 {
				continue
			}
			k := values[0]
			v := values[1]
			if r.MatchString(k) && o.Env[k] == "" {
				o.Env[k] = v
			}
		}
	}
	if o.Env["JX_VERSION"] == "" {
		c := &cmdrunner.Command{
			Name: "jx",
			Args: []string{"version", "-s"},
		}
		v, err := o.CommandRunner(c)
		if err != nil {
			return fmt.Errorf("failed to run command: %s: %w", c.CLI(), err)
		}
		v = strings.TrimSpace(v)
		if v == "" {
			log.Logger().Warnf("could not find jx version")
		} else {
			log.Logger().Infof("using jx version: %s", info(v))
			o.Env["JX_VERSION"] = v
		}
	}

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

func (o *Options) watchJob() error {
	// TODO: This should probably be rewritten inline, instead of relying on yet another tool
	args := []string{"verify", "job", "--name", o.Name, "--namespace", o.Namespace}
	if o.VerifyResult {
		args = append(args, "--verify-result")
	}
	c := &cmdrunner.Command{
		Name: "jx",
		Args: args,
		Out:  os.Stdout,
		Err:  os.Stderr,
		In:   os.Stdin,
	}
	_, err := o.CommandRunner(c)
	if err != nil {
		return fmt.Errorf("failed to run %s: %w", c.CLI(), err)
	}
	return nil
}
