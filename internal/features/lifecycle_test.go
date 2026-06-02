package features_test

import (
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/features"
)

// TestLifecycleAddPromoteRetire demonstrates the full lifecycle of a flag
// end-to-end: adding it as Experimental, promoting it to GA, and retiring it
// from the registry. Each stage is exercised through the public resolution API
// using a test-local flags struct, independent of the production registry.

// experimentalFlags is the "added" state: a brand-new experimental flag.
type experimentalFlags struct {
	MyFeature bool `lifecycle:"experimental" help:"a freshly added experimental feature"`
}

// gaFlags is the "promoted" state: the same flag, now generally available.
type gaFlags struct {
	MyFeature bool `lifecycle:"ga" help:"the feature, now generally available"`
}

// retiredFlags is the "retired" state: the flag is gone from the registry.
type retiredFlags struct {
	OtherFeature bool `lifecycle:"experimental" help:"some unrelated feature"`
}

func TestLifecycleAddPromoteRetire(t *testing.T) {
	t.Parallel()

	t.Run("add: experimental runs only when activated", func(t *testing.T) {
		t.Parallel()

		flags := &experimentalFlags{}
		reg, err := features.Bootstrap(flags)
		require.NoError(t, err)

		logger, hook := logtest.NewNullLogger()
		activated := reg.Resolve(
			features.SurfaceInput{Values: []string{"my-feature"}, Supplied: true},
			features.SurfaceInput{},
			features.SurfaceInput{},
			logger,
		)

		assert.Equal(t, []string{"my-feature"}, activated)
		assert.True(t, flags.MyFeature, "activated experimental field must be true")

		entry := lastEntry(t, hook)
		assert.Equal(t, logrus.InfoLevel, entry.Level)
		assert.Equal(t, "experimental", entry.Data["lifecycle"])
	})

	t.Run("add: experimental stays off without activation", func(t *testing.T) {
		t.Parallel()

		flags := &experimentalFlags{}
		reg, err := features.Bootstrap(flags)
		require.NoError(t, err)

		logger, _ := logtest.NewNullLogger()
		reg.Resolve(features.SurfaceInput{}, features.SurfaceInput{}, features.SurfaceInput{}, logger)

		assert.False(t, flags.MyFeature, "unactivated experimental field must stay false")
	})

	t.Run("promote: GA forced on without activation, no log, no tag", func(t *testing.T) {
		t.Parallel()

		flags := &gaFlags{}
		reg, err := features.Bootstrap(flags)
		require.NoError(t, err)

		logger, hook := logtest.NewNullLogger()
		reg.Resolve(features.SurfaceInput{}, features.SurfaceInput{}, features.SurfaceInput{}, logger)

		assert.True(t, flags.MyFeature, "GA field must be forced true even without activation")
		assert.Empty(t, reg.ActivationSet(), "unactivated GA must not enter the activation set")
		assert.Empty(t, reg.TagsForActivation(), "unactivated GA must not emit a tag")
		assert.Empty(t, hook.AllEntries(), "unactivated GA must not log")
	})

	t.Run("promote: GA activation logs remove-this-flag and emits tag", func(t *testing.T) {
		t.Parallel()

		flags := &gaFlags{}
		reg, err := features.Bootstrap(flags)
		require.NoError(t, err)

		logger, hook := logtest.NewNullLogger()
		reg.Resolve(
			features.SurfaceInput{Values: []string{"my-feature"}, Supplied: true},
			features.SurfaceInput{},
			features.SurfaceInput{},
			logger,
		)

		assert.True(t, flags.MyFeature)
		assert.Equal(t, []string{"my-feature"}, reg.ActivationSet())
		assert.Equal(t, map[string]string{"k6_feature_my_feature": "true"}, reg.TagsForActivation())

		entry := lastEntry(t, hook)
		assert.Equal(t, logrus.InfoLevel, entry.Level)
		assert.Equal(t, "ga", entry.Data["lifecycle"])
	})

	t.Run("retire: removed flag resolves Unknown, run continues", func(t *testing.T) {
		t.Parallel()

		flags := &retiredFlags{}
		reg, err := features.Bootstrap(flags)
		require.NoError(t, err)

		logger, hook := logtest.NewNullLogger()
		activated := reg.Resolve(
			features.SurfaceInput{Values: []string{"my-feature"}, Supplied: true},
			features.SurfaceInput{},
			features.SurfaceInput{},
			logger,
		)

		assert.Empty(t, activated, "retired flag must not activate")
		assert.NotContains(t, reg.ActivationSet(), "my-feature")

		entry := lastEntry(t, hook)
		assert.Equal(t, logrus.ErrorLevel, entry.Level)
		assert.Equal(t, "unknown", entry.Data["outcome"])
		assert.Equal(t, "my-feature", entry.Data["feature"])
	})
}

func lastEntry(t *testing.T, hook *logtest.Hook) *logrus.Entry {
	t.Helper()
	entry := hook.LastEntry()
	require.NotNil(t, entry)
	return entry
}
