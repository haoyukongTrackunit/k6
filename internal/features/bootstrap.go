package features

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

var kebabRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// DeriveKebabName converts a CamelCase Go field name to kebab-case.
// Example: NativeHistograms -> native-histograms.
func DeriveKebabName(fieldName string) string {
	var b strings.Builder
	for i, r := range fieldName {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func parseLifecycle(s string) (Lifecycle, bool) {
	switch s {
	case "experimental":
		return Experimental, true
	case "ga":
		return GA, true
	case "deprecated":
		return Deprecated, true
	default:
		return 0, false
	}
}

// Bootstrap reflects over Flags and builds a Registry with validated metadata.
func Bootstrap(flags *Flags) (*Registry, error) {
	t := reflect.TypeFor[Flags]()

	reg := &Registry{
		flags:  flags,
		byName: make(map[string]int),
	}

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() || f.Type.Kind() != reflect.Bool {
			continue
		}

		lcTag, ok := f.Tag.Lookup("lifecycle")
		if !ok {
			return nil, fmt.Errorf("field %s: missing lifecycle tag", f.Name)
		}
		lc, valid := parseLifecycle(lcTag)
		if !valid {
			return nil, fmt.Errorf("field %s: invalid lifecycle %q", f.Name, lcTag)
		}

		help := f.Tag.Get("help")
		if help == "" {
			return nil, fmt.Errorf("field %s: missing or empty help tag", f.Name)
		}

		name := f.Tag.Get("name")
		if name == "" {
			name = DeriveKebabName(f.Name)
		}
		if !kebabRe.MatchString(name) {
			return nil, fmt.Errorf("field %s: derived name %q is not valid kebab-case", f.Name, name)
		}

		if _, exists := reg.byName[name]; exists {
			return nil, fmt.Errorf("field %s: duplicate canonical name %q", f.Name, name)
		}

		idx := len(reg.metadata)
		reg.metadata = append(reg.metadata, Flag{
			Name:        name,
			Field:       f.Name,
			Lifecycle:   lc,
			Description: help,
		})
		reg.byName[name] = idx
	}

	return reg, nil
}
