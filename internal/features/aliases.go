package features

import (
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
)

// AliasPhase represents the migration stage of a legacy env var alias.
type AliasPhase int

const (
	// Honored means the alias env var is still accepted with a deprecation warning.
	Honored AliasPhase = iota
	// Tombstoned means the alias is recognized but rejected with an error.
	Tombstoned
	// Removed means the alias is silently ignored.
	Removed
)

// Alias maps a legacy environment variable to a canonical feature flag name.
type Alias struct {
	EnvVar    string
	Canonical string
	Phase     AliasPhase
}

// DefaultAliases returns the built-in alias definitions.
func DefaultAliases() []Alias {
	return []Alias{
		{
			EnvVar:    "K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM",
			Canonical: "native-histograms",
			Phase:     Honored,
		},
	}
}

// RegisterAlias adds a legacy env var alias to the registry.
func (r *Registry) RegisterAlias(alias Alias) error {
	if _, ok := r.byName[alias.Canonical]; !ok {
		return fmt.Errorf("alias %s: canonical name %q not in registry", alias.EnvVar, alias.Canonical)
	}
	r.aliases = append(r.aliases, alias)
	return nil
}

// ResolveEnvAliases checks registered aliases against the provided env map.
// It returns canonical names to activate and whether the env surface was
// marked as supplied by any honored alias.
func (r *Registry) ResolveEnvAliases(
	env map[string]string, logger logrus.FieldLogger,
) (names []string, supplied bool) {
	for _, a := range r.aliases {
		val, set := env[a.EnvVar]
		if !set {
			continue
		}

		switch a.Phase {
		case Honored:
			b, err := strconv.ParseBool(val)
			if err != nil {
				logger.WithFields(logrus.Fields{
					"env":   a.EnvVar,
					"value": val,
				}).Warn("Could not parse alias env var as bool, ignoring")
				continue
			}
			if !b {
				continue
			}
			logger.WithFields(logrus.Fields{
				"feature":   a.Canonical,
				"env":       a.EnvVar,
				"source":    "env_legacy_alias",
				"lifecycle": "deprecated_alias",
			}).Warn("Legacy env var detected, use --features or K6_FEATURES instead")
			names = append(names, a.Canonical)
			supplied = true

		case Tombstoned:
			logger.WithFields(logrus.Fields{
				"env":     a.EnvVar,
				"source":  "env_tombstoned_alias",
				"outcome": "unknown",
			}).Error("Legacy env var is no longer supported, use --features or K6_FEATURES instead")

		case Removed:
			// Silently ignored.
		}
	}

	return names, supplied
}

// ResolveEnvSurface builds a SurfaceInput for the env surface by combining
// the K6_FEATURES variable with any legacy alias env vars.
func (r *Registry) ResolveEnvSurface(
	k6Features string,
	k6FeaturesSet bool,
	osEnv map[string]string,
	logger logrus.FieldLogger,
) SurfaceInput {
	aliasNames, aliasSupplied := r.ResolveEnvAliases(osEnv, logger)

	var featureNames []string
	if k6FeaturesSet {
		featureNames = ParseInput(k6Features)
	}

	// Union alias names with K6_FEATURES names.
	seen := make(map[string]struct{}, len(aliasNames)+len(featureNames))
	var values []string
	for _, n := range aliasNames {
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			values = append(values, n)
		}
	}
	for _, n := range featureNames {
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			values = append(values, n)
		}
	}

	return SurfaceInput{
		Values:   values,
		Supplied: k6FeaturesSet || aliasSupplied,
	}
}
