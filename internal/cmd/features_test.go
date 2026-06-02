package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd/tests"
)

func TestFeaturesSubCommand(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "features"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	require.NotEmpty(t, stdout)

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	require.NotEmpty(t, lines)
	assert.Contains(t, lines[0], "NAME")
	assert.Contains(t, lines[0], "LIFECYCLE")
	assert.Contains(t, lines[0], "DESCRIPTION")
	assert.Contains(t, stdout, "native-histograms")
	assert.Contains(t, stdout, "Experimental")
	assert.Contains(t, stdout, "Use native histograms for trend metrics")
}

func TestFeaturesJSONSubCommand(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "features", "--json"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	require.NotEmpty(t, stdout)

	var flags []struct {
		Name        string `json:"name"`
		Lifecycle   string `json:"lifecycle"`
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &flags))
	require.NotEmpty(t, flags)

	foundNativeHistograms := false
	for _, flag := range flags {
		assert.NotEmpty(t, flag.Name)
		assert.NotEmpty(t, flag.Lifecycle)
		assert.NotEmpty(t, flag.Description)

		if flag.Name == "native-histograms" {
			foundNativeHistograms = true
			assert.Equal(t, "Experimental", flag.Lifecycle)
			assert.Equal(t, "Use native histograms for trend metrics", flag.Description)
		}
	}

	assert.True(t, foundNativeHistograms)
}
