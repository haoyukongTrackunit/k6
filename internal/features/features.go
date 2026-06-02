// Package features implements a feature flag system for k6.
package features

import (
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

// Lifecycle represents the maturity stage of a feature flag.
type Lifecycle int

const (
	// Experimental flags are opt-in and may change without notice.
	Experimental Lifecycle = iota
	// GA flags are generally available and enabled by default.
	GA
	// Deprecated flags are scheduled for removal.
	Deprecated
)

// String returns the lowercase lifecycle name.
func (l Lifecycle) String() string {
	switch l {
	case Experimental:
		return "experimental"
	case GA:
		return "ga"
	case Deprecated:
		return "deprecated"
	default:
		return "unknown"
	}
}

// DisplayString returns the title-case lifecycle name.
func (l Lifecycle) DisplayString() string {
	switch l {
	case Experimental:
		return "Experimental"
	case GA:
		return "GA"
	case Deprecated:
		return "Deprecated"
	default:
		return "Unknown"
	}
}

// Flags holds every feature flag as an exported bool field.
// Struct tags carry metadata: lifecycle (required), help (required), name (optional override).
type Flags struct {
	NativeHistograms bool `lifecycle:"experimental" help:"Use native histograms for trend metrics"`
}

// Flag describes a single feature flag's metadata.
type Flag struct {
	Name        string
	Field       string // Go struct field name
	Lifecycle   Lifecycle
	Description string
}

// Registry holds the flag state plus metadata built by Bootstrap.
type Registry struct {
	flags     *Flags
	metadata  []Flag
	byName    map[string]int // canonical name -> metadata index
	activated []string       // canonical names after resolution
	aliases   []Alias
}

// lifecycleOrder defines the display sort priority.
func lifecycleOrder(l Lifecycle) int {
	switch l {
	case Experimental:
		return 0
	case Deprecated:
		return 1
	case GA:
		return 2
	default:
		return 3
	}
}

// All returns all flags sorted for display: Experimental, Deprecated, GA,
// alphabetical within each group.
func (r *Registry) All() []Flag {
	out := make([]Flag, len(r.metadata))
	copy(out, r.metadata)
	sort.Slice(out, func(i, j int) bool {
		oi, oj := lifecycleOrder(out[i].Lifecycle), lifecycleOrder(out[j].Lifecycle)
		if oi != oj {
			return oi < oj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// ActivationSet returns the sorted canonical names that were activated
// during the last Resolve call.
func (r *Registry) ActivationSet() []string {
	out := make([]string, len(r.activated))
	copy(out, r.activated)
	return out
}

// TagsForActivation returns k6 metric tags for each activated flag.
// Key format: k6_feature_<snake_case_name>, value: "true".
func (r *Registry) TagsForActivation() map[string]string {
	tags := make(map[string]string, len(r.activated))
	for _, name := range r.activated {
		snake := strings.ReplaceAll(name, "-", "_")
		tags["k6_feature_"+snake] = "true"
	}
	return tags
}

// Flags returns the underlying Flags struct for direct field reads.
func (r *Registry) Flags() *Flags {
	return r.flags
}

// Global is the process-wide feature flags instance.
var Global = &Flags{} //nolint:gochecknoglobals

// Init bootstraps the registry, registers the default aliases, and resolves the
// activation set from the three configuration surfaces. The env map carries
// K6_FEATURES plus any legacy alias env vars; the env surface is built from it
// internally. It returns the resolved registry, ready for field reads and tags.
func Init(logger logrus.FieldLogger, cli, json SurfaceInput, env map[string]string) (*Registry, error) {
	reg, err := Bootstrap(Global)
	if err != nil {
		return nil, err
	}

	for _, a := range DefaultAliases() {
		if err := reg.RegisterAlias(a); err != nil {
			return nil, err
		}
	}

	k6f, k6fSet := env["K6_FEATURES"]
	envSurface := reg.ResolveEnvSurface(k6f, k6fSet, env, logger)

	reg.Resolve(cli, envSurface, json, logger)
	return reg, nil
}
