package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/features"
)

type featuresCmd struct {
	gs     *state.GlobalState
	isJSON bool
}

func (c *featuresCmd) run(_ *cobra.Command, _ []string) error {
	reg, err := features.Bootstrap(&features.Flags{})
	if err != nil {
		return fmt.Errorf("bootstrap feature registry: %w", err)
	}

	all := reg.All()
	sortFlags(all)

	if c.isJSON {
		return c.printJSON(all)
	}
	return c.printTable(all)
}

func (c *featuresCmd) printTable(flags []features.Flag) error {
	if len(flags) == 0 {
		return nil
	}

	w := tabwriter.NewWriter(c.gs.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tLIFECYCLE\tDESCRIPTION"); err != nil {
		return err
	}
	for _, f := range flags {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", f.Name, f.Lifecycle.DisplayString(), f.Description); err != nil {
			return err
		}
	}
	return w.Flush()
}

func (c *featuresCmd) printJSON(flags []features.Flag) error {
	type jsonFlag struct {
		Name        string `json:"name"`
		Lifecycle   string `json:"lifecycle"`
		Description string `json:"description"`
	}

	out := make([]jsonFlag, len(flags))
	for i, f := range flags {
		out[i] = jsonFlag{
			Name:        f.Name,
			Lifecycle:   f.Lifecycle.DisplayString(),
			Description: f.Description,
		}
	}

	enc := json.NewEncoder(c.gs.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// sortFlags orders by lifecycle (Experimental, Deprecated, GA), then alphabetically within each group.
func sortFlags(flags []features.Flag) {
	lifecycleOrder := map[features.Lifecycle]int{
		features.Experimental: 0,
		features.Deprecated:   1,
		features.GA:           2,
	}
	sort.Slice(flags, func(i, j int) bool {
		oi, oj := lifecycleOrder[flags[i].Lifecycle], lifecycleOrder[flags[j].Lifecycle]
		if oi != oj {
			return oi < oj
		}
		return flags[i].Name < flags[j].Name
	})
}

func getCmdFeatures(gs *state.GlobalState) *cobra.Command {
	c := &featuresCmd{gs: gs}

	cmd := &cobra.Command{
		Use:   "features",
		Short: "List available feature flags",
		Long:  `List all available feature flags with their lifecycle status and description.`,
		RunE:  c.run,
	}

	cmd.Flags().BoolVar(&c.isJSON, "json", false, "output in JSON format")

	return cmd
}
