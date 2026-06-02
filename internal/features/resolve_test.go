package features_test

import (
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/features"
)

func TestRegistryResolveWinnerTakesAllPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cli  features.SurfaceInput
		env  features.SurfaceInput
		json features.SurfaceInput
		want []string
	}{
		{
			name: "cli beats env and json",
			cli:  features.SurfaceInput{Values: []string{"native-histograms"}, Supplied: true},
			env:  features.SurfaceInput{Supplied: true},
			json: features.SurfaceInput{Supplied: true},
			want: []string{"native-histograms"},
		},
		{
			name: "env beats json",
			env:  features.SurfaceInput{Values: []string{"native-histograms"}, Supplied: true},
			json: features.SurfaceInput{Supplied: true},
			want: []string{"native-histograms"},
		},
		{
			name: "json used when higher priority absent",
			json: features.SurfaceInput{Values: []string{"native-histograms"}, Supplied: true},
			want: []string{"native-histograms"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := newRegistry(t)
			logger, _ := logtest.NewNullLogger()

			activated := reg.Resolve(tt.cli, tt.env, tt.json, logger)

			assert.Equal(t, tt.want, activated)
			assert.Equal(t, tt.want, reg.ActivationSet())
			assert.True(t, reg.Flags().NativeHistograms)
		})
	}
}

func TestRegistryResolveEmptyCLIOverride(t *testing.T) {
	t.Parallel()

	reg := newRegistry(t)
	logger, _ := logtest.NewNullLogger()

	activated := reg.Resolve(
		features.SurfaceInput{Supplied: true},
		features.SurfaceInput{Values: []string{"native-histograms"}, Supplied: true},
		features.SurfaceInput{Values: []string{"native-histograms"}, Supplied: true},
		logger,
	)

	assert.Empty(t, activated)
	assert.Empty(t, reg.ActivationSet())
	assert.False(t, reg.Flags().NativeHistograms)
}

func TestRegistryResolveUnknownName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cli    features.SurfaceInput
		env    features.SurfaceInput
		json   features.SurfaceInput
		source string
	}{
		{
			name:   "cli source",
			cli:    features.SurfaceInput{Values: []string{"not-a-flag"}, Supplied: true},
			source: "cli",
		},
		{
			name:   "env source",
			env:    features.SurfaceInput{Values: []string{"not-a-flag"}, Supplied: true},
			source: "env",
		},
		{
			name:   "json source",
			json:   features.SurfaceInput{Values: []string{"not-a-flag"}, Supplied: true},
			source: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := newRegistry(t)
			logger, hook := logtest.NewNullLogger()

			assert.NotPanics(t, func() {
				reg.Resolve(tt.cli, tt.env, tt.json, logger)
			})

			assert.NotContains(t, reg.ActivationSet(), "not-a-flag")
			assert.False(t, reg.Flags().NativeHistograms)

			entry := assertSingleLogEntry(t, hook, logrus.ErrorLevel)
			assert.Equal(t, "not-a-flag", entry.Data["feature"])
			assert.Equal(t, "unknown", entry.Data["outcome"])
			assert.Equal(t, tt.source, entry.Data["source"])
		})
	}
}

func TestRegistryResolveInvalidNameIsUnknown(t *testing.T) {
	t.Parallel()

	// Non-kebab user input must be treated as Unknown (outcome "unknown"),
	// not a distinct "invalid" outcome.
	for _, name := range []string{"Native-Histograms", "foo_bar", "foo--bar"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			reg := newRegistry(t)
			logger, hook := logtest.NewNullLogger()

			activated := reg.Resolve(
				features.SurfaceInput{Values: []string{name}, Supplied: true},
				features.SurfaceInput{},
				features.SurfaceInput{},
				logger,
			)

			assert.Empty(t, activated)

			entry := assertSingleLogEntry(t, hook, logrus.ErrorLevel)
			assert.Equal(t, name, entry.Data["feature"])
			assert.Equal(t, "unknown", entry.Data["outcome"])
			assert.Equal(t, "cli", entry.Data["source"])
		})
	}
}

func TestRegistryResolveRecognizedExperimental(t *testing.T) {
	t.Parallel()

	reg := newRegistry(t)
	logger, hook := logtest.NewNullLogger()

	activated := reg.Resolve(
		features.SurfaceInput{Values: []string{"native-histograms"}, Supplied: true},
		features.SurfaceInput{},
		features.SurfaceInput{},
		logger,
	)

	require.Equal(t, []string{"native-histograms"}, activated)
	assert.True(t, reg.Flags().NativeHistograms)

	entry := assertSingleLogEntry(t, hook, logrus.InfoLevel)
	assert.Equal(t, "native-histograms", entry.Data["feature"])
	assert.Equal(t, "experimental", entry.Data["lifecycle"])
}

func TestRegistryResolveNoSurfaceSupplied(t *testing.T) {
	t.Parallel()

	reg := newRegistry(t)
	logger, hook := logtest.NewNullLogger()

	activated := reg.Resolve(features.SurfaceInput{}, features.SurfaceInput{}, features.SurfaceInput{}, logger)

	assert.Empty(t, activated)
	assert.Empty(t, reg.ActivationSet())
	assert.False(t, reg.Flags().NativeHistograms)
	assert.Empty(t, hook.AllEntries())
}
