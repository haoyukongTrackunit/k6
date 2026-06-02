package features_test

import (
	"slices"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/features"
)

const nativeHistogramsAliasEnv = "K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM"

func TestDefaultAliases(t *testing.T) {
	t.Parallel()

	assert.True(t, slices.ContainsFunc(features.DefaultAliases(), func(alias features.Alias) bool {
		return alias.EnvVar == nativeHistogramsAliasEnv &&
			alias.Canonical == "native-histograms" &&
			alias.Phase == features.Honored
	}))
}

func TestRegistryResolveEnvAliasesHonoredTruthy(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"true", "1", "TRUE"} {
		t.Run("value="+value, func(t *testing.T) {
			t.Parallel()

			reg := newRegistryWithDefaultAliases(t)
			env := map[string]string{nativeHistogramsAliasEnv: value}

			logger, hook := logtest.NewNullLogger()
			names, supplied := reg.ResolveEnvAliases(env, logger)

			assert.Equal(t, []string{"native-histograms"}, names)
			assert.True(t, supplied)

			entry := assertSingleLogEntry(t, hook, logrus.WarnLevel)
			assert.Equal(t, "native-histograms", entry.Data["feature"])
			assert.Equal(t, nativeHistogramsAliasEnv, entry.Data["env"])
			assert.Equal(t, "env_legacy_alias", entry.Data["source"])
			assert.Equal(t, "deprecated_alias", entry.Data["lifecycle"])

			logger, hook = logtest.NewNullLogger()
			surface := reg.ResolveEnvSurface("", false, env, logger)

			assert.Equal(t, []string{"native-histograms"}, surface.Values)
			assert.True(t, surface.Supplied)

			entry = assertSingleLogEntry(t, hook, logrus.WarnLevel)
			assert.Equal(t, "native-histograms", entry.Data["feature"])
			assert.Equal(t, nativeHistogramsAliasEnv, entry.Data["env"])
			assert.Equal(t, "env_legacy_alias", entry.Data["source"])
			assert.Equal(t, "deprecated_alias", entry.Data["lifecycle"])
		})
	}
}

func TestRegistryResolveEnvAliasesHonoredFalsy(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"false", "0"} {
		t.Run("value="+value, func(t *testing.T) {
			t.Parallel()

			reg := newRegistryWithDefaultAliases(t)
			env := map[string]string{nativeHistogramsAliasEnv: value}

			logger, hook := logtest.NewNullLogger()
			names, supplied := reg.ResolveEnvAliases(env, logger)

			assert.Empty(t, names)
			assert.False(t, supplied)
			assert.Empty(t, hook.AllEntries())

			logger, hook = logtest.NewNullLogger()
			surface := reg.ResolveEnvSurface("", false, env, logger)

			assert.Empty(t, surface.Values)
			assert.False(t, surface.Supplied)
			assert.Empty(t, hook.AllEntries())
		})
	}
}

func TestRegistryResolveEnvAliasesTombstoned(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"1", "false"} {
		t.Run("value="+value, func(t *testing.T) {
			t.Parallel()

			reg := newRegistry(t)
			require.NoError(t, reg.RegisterAlias(features.Alias{
				EnvVar:    "K6_TEST_TOMBSTONED_ALIAS",
				Canonical: "native-histograms",
				Phase:     features.Tombstoned,
			}))
			env := map[string]string{"K6_TEST_TOMBSTONED_ALIAS": value}

			logger, hook := logtest.NewNullLogger()
			names, supplied := reg.ResolveEnvAliases(env, logger)

			assert.Empty(t, names)
			assert.False(t, supplied)

			entry := assertSingleLogEntry(t, hook, logrus.ErrorLevel)
			assert.Equal(t, "K6_TEST_TOMBSTONED_ALIAS", entry.Data["env"])
			assert.Equal(t, "env_tombstoned_alias", entry.Data["source"])
			assert.Equal(t, "unknown", entry.Data["outcome"])

			logger, hook = logtest.NewNullLogger()
			surface := reg.ResolveEnvSurface("", false, env, logger)

			assert.Empty(t, surface.Values)
			assert.False(t, surface.Supplied)

			entry = assertSingleLogEntry(t, hook, logrus.ErrorLevel)
			assert.Equal(t, "K6_TEST_TOMBSTONED_ALIAS", entry.Data["env"])
			assert.Equal(t, "env_tombstoned_alias", entry.Data["source"])
			assert.Equal(t, "unknown", entry.Data["outcome"])
		})
	}
}

func TestRegistryResolveEnvSurfaceUnionDedupesNames(t *testing.T) {
	t.Parallel()

	reg := newRegistryWithDefaultAliases(t)
	logger, hook := logtest.NewNullLogger()
	env := map[string]string{nativeHistogramsAliasEnv: "true"}

	surface := reg.ResolveEnvSurface("native-histograms", true, env, logger)

	assert.Equal(t, []string{"native-histograms"}, surface.Values)
	assert.True(t, surface.Supplied)

	entry := assertSingleLogEntry(t, hook, logrus.WarnLevel)
	assert.Equal(t, "env_legacy_alias", entry.Data["source"])
	assert.Equal(t, "deprecated_alias", entry.Data["lifecycle"])
}

func TestRegistryRegisterAliasErrorsForUnknownCanonical(t *testing.T) {
	t.Parallel()

	reg := newRegistry(t)

	err := reg.RegisterAlias(features.Alias{
		EnvVar:    "K6_TEST_UNKNOWN_ALIAS",
		Canonical: "not-a-flag",
		Phase:     features.Honored,
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, `canonical name "not-a-flag" not in registry`)
}

func newRegistryWithDefaultAliases(t *testing.T) *features.Registry {
	t.Helper()

	reg := newRegistry(t)
	for _, alias := range features.DefaultAliases() {
		require.NoError(t, reg.RegisterAlias(alias))
	}

	return reg
}
