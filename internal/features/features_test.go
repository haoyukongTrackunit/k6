package features_test

import (
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/features"
)

func TestDeriveKebabName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "camel case", input: "NativeHistograms", want: "native-histograms"},
		{name: "single word", input: "Feature", want: "feature"},
		{name: "already lower", input: "native", want: "native"},
		{name: "single letter", input: "X", want: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, features.DeriveKebabName(tt.input))
		})
	}
}

func TestLifecycleStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		lifecycle   features.Lifecycle
		wantString  string
		wantDisplay string
	}{
		{
			name:        "experimental",
			lifecycle:   features.Experimental,
			wantString:  "experimental",
			wantDisplay: "Experimental",
		},
		{
			name:        "ga",
			lifecycle:   features.GA,
			wantString:  "ga",
			wantDisplay: "GA",
		},
		{
			name:        "deprecated",
			lifecycle:   features.Deprecated,
			wantString:  "deprecated",
			wantDisplay: "Deprecated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantString, tt.lifecycle.String())
			assert.Equal(t, tt.wantDisplay, tt.lifecycle.DisplayString())
		})
	}
}

func TestBootstrap(t *testing.T) {
	t.Parallel()

	reg := newRegistry(t)
	all := reg.All()
	require.NotEmpty(t, all)

	nativeHistograms, ok := findFlag(all, "native-histograms")
	require.True(t, ok)
	assert.Equal(t, features.Experimental, nativeHistograms.Lifecycle)
	assert.NotEmpty(t, nativeHistograms.Description)

	for i := 1; i < len(all); i++ {
		prev := all[i-1]
		curr := all[i]
		prevOrder := lifecycleSortOrder(prev.Lifecycle)
		currOrder := lifecycleSortOrder(curr.Lifecycle)

		assert.Truef(
			t,
			prevOrder <= currOrder,
			"flags out of lifecycle order: %s (%s) before %s (%s)",
			prev.Name,
			prev.Lifecycle.DisplayString(),
			curr.Name,
			curr.Lifecycle.DisplayString(),
		)
		if prevOrder == currOrder {
			assert.Truef(
				t,
				prev.Name <= curr.Name,
				"flags out of alphabetical order within lifecycle: %s before %s",
				prev.Name,
				curr.Name,
			)
		}
	}
}

func TestParseInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "comma separated", raw: "one,two", want: []string{"one", "two"}},
		{name: "trim whitespace", raw: " one , two , three ", want: []string{"one", "two", "three"}},
		{name: "drop empties", raw: ",one,, ,two,", want: []string{"one", "two"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, features.ParseInput(tt.raw))
		})
	}

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, features.ParseInput(""))
	})
}

func TestTagsForActivation(t *testing.T) {
	t.Parallel()

	reg := newRegistry(t)
	logger, _ := logtest.NewNullLogger()

	activated := reg.Resolve(
		features.SurfaceInput{Values: []string{"native-histograms"}, Supplied: true},
		features.SurfaceInput{},
		features.SurfaceInput{},
		logger,
	)

	assert.Equal(t, []string{"native-histograms"}, activated)
	assert.Equal(t, map[string]string{
		"k6_feature_native_histograms": "true",
	}, reg.TagsForActivation())
}

func newRegistry(t *testing.T) *features.Registry {
	t.Helper()

	reg, err := features.Bootstrap(&features.Flags{})
	require.NoError(t, err)

	return reg
}

func findFlag(flags []features.Flag, name string) (features.Flag, bool) {
	for _, flag := range flags {
		if flag.Name == name {
			return flag, true
		}
	}

	return features.Flag{}, false
}

func lifecycleSortOrder(lifecycle features.Lifecycle) int {
	switch lifecycle {
	case features.Experimental:
		return 0
	case features.Deprecated:
		return 1
	case features.GA:
		return 2
	default:
		return 3
	}
}

func assertSingleLogEntry(t *testing.T, hook *logtest.Hook, level logrus.Level) *logrus.Entry {
	t.Helper()

	entries := hook.AllEntries()
	require.Len(t, entries, 1)

	entry := hook.LastEntry()
	require.NotNil(t, entry)
	assert.Equal(t, level, entry.Level)

	return entry
}
