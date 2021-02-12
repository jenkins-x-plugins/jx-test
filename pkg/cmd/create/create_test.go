package create_test

import (
	"strconv"
	"testing"

	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner/fakerunner"
	"github.com/jenkins-x/jx-test/pkg/cmd/create"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	owner := "myowner"
	repo := "myrepo"
	contextName := "myctx"
	prNumber := 456
	prLabel := "pr-" + strconv.Itoa(prNumber)

	runner := &fakerunner.FakeRunner{}

	_, o := create.NewCmdCreate()

	o.PullRequestNumber = prNumber
	o.RepoOwner = owner
	o.RepoName = repo
	o.Context = contextName
	o.BuildNumber = "3"
	o.File = "foo"
	o.CommandRunner = runner.Run

	err := o.Run()
	require.NoError(t, err, "failed to run create command")

	assert.Equal(t, "myrepo-pr456-myctx-3", o.ResourceName, "o.ResourceName")
	assert.Equal(t, map[string]string{"context": contextName, "owner": owner, "pr": prLabel, "repo": repo}, o.Labels, "o.Labels")

	for _, c := range runner.OrderedCommands {
		t.Logf("faked: %s\n", c.CLI())
	}
}
