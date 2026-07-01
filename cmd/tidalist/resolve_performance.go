package main

import (
	"fmt"

	"github.com/cehbz/tidalist/catalog"
	"github.com/spf13/cobra"
)

func newResolvePerformanceCmd() *cobra.Command {
	var work, label, catno string
	var creditSpecs []string
	var year, limit int
	cmd := &cobra.Command{
		Use:   "resolve-performance",
		Short: "Resolve a classical item to a federated performance (MB + Discogs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if work == "" {
				return fmt.Errorf("provide --work")
			}
			credits, err := parseCredits(creditSpecs)
			if err != nil {
				return err
			}
			m, err := openMirror(cmd)
			if err != nil {
				return err
			}
			defer m.Close()
			res, err := m.ResolvePerformance(catalog.PerformanceQuery{
				Work:    work,
				Credits: credits,
				Year:    year,
				Label:   label,
				Catno:   catno,
				Limit:   limit,
			})
			if err != nil {
				return err
			}
			if res.Performances == nil {
				res.Performances = []catalog.Performance{}
			}
			return emitJSON(cmd, res)
		},
	}
	cmd.Flags().StringVar(&work, "work", "", "work title (required)")
	cmd.Flags().StringArrayVar(&creditSpecs, "credit", nil, "role:name[:attrs] (repeatable; incl. composer:)")
	cmd.Flags().IntVar(&year, "year", 0, "within-block vintage selector")
	cmd.Flags().StringVar(&label, "label", "", "within-block label selector")
	cmd.Flags().StringVar(&catno, "catno", "", "within-block catalog-number selector")
	cmd.Flags().IntVar(&limit, "limit", 25, "max performances to surface")
	return cmd
}
