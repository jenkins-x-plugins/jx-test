package create

import (
	"context"
	"fmt"
	"github.com/jenkins-x/jx-test/pkg/dynkube"
	"github.com/jenkins-x/jx-test/pkg/terraforms"
	"regexp"

	"github.com/Masterminds/sprig/v3"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-helpers/v3/pkg/pipelinectx"
	"github.com/jenkins-x/jx-helpers/v3/pkg/templater"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-kube-client/v3/pkg/kubeclient"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/jenkins-x/jx-test/pkg/root"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io/ioutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"os"
	"sigs.k8s.io/yaml"
	"strings"
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
	Env              map[string]string
	EnvVars          []string
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
		Run: func(cmd *cobra.Command, args []string) {
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
	return cmd, o
}

// Run implements the command
func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate")
	}

	log.Logger().Infof("resource: %s", info(o.ResourceName))
	log.Logger().Infof("labels: %v", o.Labels)

	templateText, err := ioutil.ReadFile(o.File)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", o.File)
	}

	o.Name = o.ResourceName
	funcMap := sprig.TxtFuncMap()

	output, err := templater.Evaluate(funcMap, o, string(templateText), o.File, "resource template")
	if err != nil {
		return errors.Wrapf(err, "failed to evaluate template %s", o.File)
	}

	if o.LogResource {
		log.Logger().Infof("generated template: %s", output)
	}

	u := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(output), u)
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal the template of file %s has YAML: %s", o.File, output)
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
		return errors.Errorf("generated template of file %s has missing kind", o.File)
	}
	if apiVersion == "" {
		return errors.Errorf("generated template of file %s has missing apiVersion", o.File)
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to parse apiVersion: %s", apiVersion)
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
		return errors.Wrapf(err, "could not find resources for ")
	}
	if list != nil {
		for _, r := range list.Items {
			name := r.GetName()

			err = dynkube.DynamicResource(o.DynamicClient, ns, gvr).Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil {
				return errors.Wrapf(err, "failed to delete %s", name)
			}
			log.Logger().Infof("deleted previous pipeline %s %s", kind, info(name))
		}
	}

	// now lets create the new resource
	name := o.Name
	if name == "" {
		return errors.Errorf("no name defaulted")
	}

	_, err = dynkube.DynamicResource(o.DynamicClient, ns, gvr).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return errors.Errorf("should not have a %s called %s", kind, name)
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to check if %s %s exists", kind, name)
	}

	u, err = dynkube.DynamicResource(o.DynamicClient, ns, gvr).Create(ctx, u, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to create %s %s", kind, name)
	}
	log.Logger().Infof("created %s %s", kind, info(name))

	if o.NoWatchJob {
		return nil
	}
	err = o.watchJob()
	if err != nil {
		return errors.Wrapf(err, "job failed to complete succesfully")
	}

	if o.NoDeleteResource {
		return nil
	}

	err = dynkube.DynamicResource(o.DynamicClient, ns, gvr).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to delete %s %s", kind, name)
	}
	log.Logger().Infof("Job succeeded so deleted %s %s", kind, info(name))
	return nil
}

// Validate validates options
func (o *Options) Validate() error {
	err := o.Options.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate pipeline options")
	}

	if o.File == "" {
		return options.MissingOption("file")
	}
	exists, err := files.FileExists(o.File)
	if err != nil {
		return errors.Wrapf(err, "failed to check if file exists %s", o.File)
	}
	if !exists {
		return errors.Errorf("file %s does not exist", o.File)
	}

	// lets delete any old resources
	if len(o.Labels) == 0 {
		return errors.Errorf("no labels could be created")
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
			return errors.Wrapf(err, "failed to parse option --env-pattern %s", o.EnvPattern)
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

	o.DynamicClient, err = kube.LazyCreateDynamicClient(o.DynamicClient)
	if o.Namespace == "" {
		o.Namespace, err = kubeclient.CurrentNamespace()
		if err != nil {
			return errors.Wrap(err, "failed to get current kubernetes namespace")
		}
	}
	if o.CommandRunner == nil {
		o.CommandRunner = cmdrunner.DefaultCommandRunner
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
	c := &cmdrunner.Command{
		Name: "jx",
		Args: []string{"verify", "job", "--name", o.Name, "--namespace", o.Namespace},
		Out:  os.Stdout,
		Err:  os.Stderr,
		In:   os.Stdin,
	}
	_, err := o.CommandRunner(c)
	if err != nil {
		return errors.Wrapf(err, "failed to run %s", c.CLI())
	}
	return nil
}
