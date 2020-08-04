package v1alpha1

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TestRun represents an instance of a system/integration/BDD test on kubernetes
//
// +k8s:openapi-gen=true
type TestRun struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the TestRun from the client
	// +optional
	Spec TestRunSpec `json:"spec"`

	// Status holds the current status
	// +optional
	Status *TestStatus `json:"status,omitempty"`
}

// TestRunSpec defines the desired state of TestRun.
type TestRunSpec struct {
	// TestSource the gitops repository used to define the test resources
	TestSource Source `json:"testSource,omitempty"`

	// RemoveScript is the script inside the test source git repository that is used to remove the test resources
	RemoveScript string `json:"removeScript,omitempty"`

	// TriggerSource the git repository which triggered the test
	TriggerSource Source `json:"triggerSource,omitempty"`

	// Branch is the branch name the test is being run from such as PR-1234
	Branch string `json:"branch,omitempty"`

	// Context is the pipeline context for the test. There could be many contexts per repository
	// such as 'gke-vault', 'gke-gsm', 'eks-vault'
	Context string `json:"context,omitempty"`

	// BuildNumber the build number
	BuildNumber int `json:"buildNumber,omitempty"`

	// Env the environment variables for the test
	Env map[string]string `json:"env,omitempty"`

	// Delete marks the test for deletion on the next garbage collection run
	Delete bool `json:"delete"`

	// Keep disables the usual garbage collection
	Keep bool `json:"keep"`
}

// TestKind returns a string which is unique for a trigger source repository,
// context and branch
func (t *TestRunSpec) TestKind() string {
	return strings.Join([]string{t.TriggerSource.URL, t.Branch, t.Context}, "/")
}

// Validate populates any missing values from environment variables
func (t *TestRunSpec) Validate() error {
	if t.TestSource.URL == "" {
		return errors.Errorf("missing spec.testSource.url")
	}
	// lets default details of the repository and branch we are creating the test from
	if t.TriggerSource.URL == "" {
		t.TriggerSource.URL = t.Env["SOURCE_URL"]
		if t.TriggerSource.URL == "" {
			return errors.Errorf("no $SOURCE_URL value")
		}
	}
	if t.Branch == "" {
		t.Branch = t.Env["BRANCH_NAME"]
		if t.Branch == "" {
			return errors.Errorf("no $BRANCH_NAME value")
		}
	}
	if t.Context == "" {
		t.Context = t.Env["PIPELINE_CONTEXT"]
		if t.Branch == "" {
			return errors.Errorf("no $PIPELINE_CONTEXT value")
		}
	}
	if t.BuildNumber == 0 {
		buildText := t.Env["BUILD_NUMBER"]
		if buildText == "" {
			return errors.Errorf("no $BUILD_NUMBER value")
		}
		var err error
		t.BuildNumber, err = strconv.Atoi(buildText)
		if err != nil {
			return errors.Wrapf(err, "failed to parse $BUILD_NUMBER value %s", buildText)
		}
	}
	return nil
}

// Source defines a git repository.
type Source struct {
	URL string `json:"url,omitempty"`
	Ref string `json:"ref,omitempty"`
}

// TestStatus defines the current status of the TestRun.
type TestStatus struct {
	Status string `json:"status,omitempty"`
}

// TestRunList contains a list of TestRun
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TestRunList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TestRun `json:"items"`
}
