package main

import (
	"fmt"

	"github.com/cehbz/tidalist/catalog"
	"github.com/spf13/cobra"
)

func newFindByAttributesCmd() *cobra.Command {
	var styles, genres []string
	var yearFrom, yearTo, limit int
	cmd := &cobra.Command{
		Use:   "find-by-attributes",
		Short: "Find Discogs masters by style/genre/year descriptors (no title needed)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(styles) == 0 && len(genres) == 0 {
				return fmt.Errorf("provide at least one --style or --genre")
			}
			q := catalog.AttributeQuery{
				Styles:   styles,
				Genres:   genres,
				YearFrom: yearFrom,
				YearTo:   yearTo,
				Limit:    limit,
			}
			m, err := openMirror(cmd)
			if err != nil {
				return err
			}
			defer m.Close()
			cands, err := m.FindByAttributes(q)
			if err != nil {
				return err
			}
			if cands == nil {
				cands = []catalog.AttributeCandidate{}
			}
			return emitJSON(cmd, map[string]any{"candidates": cands})
		},
	}
	cmd.Flags().StringArrayVar(&styles, "style", nil, "Discogs style, AND-ed across repeats (repeatable)")
	cmd.Flags().StringArrayVar(&genres, "genre", nil, "Discogs genre, AND-ed across repeats (repeatable)")
	cmd.Flags().IntVar(&yearFrom, "year-from", 0, "earliest year (inclusive)")
	cmd.Flags().IntVar(&yearTo, "year-to", 0, "latest year (inclusive)")
	cmd.Flags().IntVar(&limit, "limit", 25, "max candidates")
	return cmd
}
