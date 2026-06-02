package features

import (
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

// SurfaceInput represents one configuration surface (CLI, env, JSON).
type SurfaceInput struct {
	Values   []string // comma-split, trimmed feature names
	Supplied bool     // whether the surface was supplied at all
}

// ParseInput splits a raw comma-separated string into trimmed, non-empty names.
func ParseInput(raw string) []string {
	var out []string
	for s := range strings.SplitSeq(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Resolve picks the highest-priority supplied surface and activates flags.
// Priority: CLI > env > JSON.
func (r *Registry) Resolve(cli, env, json SurfaceInput, logger logrus.FieldLogger) []string {
	var winner SurfaceInput
	var source string

	switch {
	case cli.Supplied:
		winner, source = cli, "cli"
	case env.Supplied:
		winner, source = env, "env"
	case json.Supplied:
		winner, source = json, "json"
	default:
		// No surface supplied — only force GA flags.
		r.forceGA()
		return nil
	}

	names := parseValues(winner.Values)

	activated := make(map[string]struct{})

	for _, name := range names {
		if !kebabRe.MatchString(name) {
			logger.WithFields(logrus.Fields{
				"feature": name,
				"outcome": "invalid",
				"source":  source,
			}).Error("Feature flag name is not valid kebab-case")
			continue
		}

		idx, known := r.byName[name]
		if !known {
			logger.WithFields(logrus.Fields{
				"feature": name,
				"outcome": "unknown",
				"source":  source,
			}).Error("Unknown feature flag")
			continue
		}

		meta := r.metadata[idx]
		activated[name] = struct{}{}

		switch meta.Lifecycle {
		case Experimental:
			logger.WithFields(logrus.Fields{
				"feature":   name,
				"lifecycle": "experimental",
			}).Info("Experimental feature enabled")
		case GA:
			logger.WithFields(logrus.Fields{
				"feature":   name,
				"lifecycle": "ga",
			}).Info("Feature is now available by default, please remove this flag")
		case Deprecated:
			logger.WithFields(logrus.Fields{
				"feature":   name,
				"lifecycle": "deprecated",
			}).Warn("Deprecated feature enabled")
		}
	}

	// Set activated fields on the Flags struct.
	r.setFields(activated)

	// Force all GA fields true regardless of activation.
	r.forceGA()

	// Build sorted activation set.
	r.activated = sortedKeys(activated)

	return r.ActivationSet()
}

// parseValues splits and trims comma-separated values, dropping empties.
func parseValues(vals []string) []string {
	var out []string
	for _, v := range vals {
		for s := range strings.SplitSeq(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func (r *Registry) setFields(names map[string]struct{}) {
	for name := range names {
		idx := r.byName[name]
		r.target.FieldByName(r.metadata[idx].Field).SetBool(true)
	}
}

func (r *Registry) forceGA() {
	for _, meta := range r.metadata {
		if meta.Lifecycle == GA {
			r.target.FieldByName(meta.Field).SetBool(true)
		}
	}
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
