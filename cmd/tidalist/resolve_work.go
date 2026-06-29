package main

import (
	"github.com/cehbz/tidalist/catalog"
	"github.com/spf13/cobra"
)

func newResolveWorkCmd() *cobra.Command {
	var name string
	var limit int
	cmd := &cobra.Command{
		Use:   "resolve-work",
		Short: "Resolve a work (composition) name to ranked MusicBrainz candidates",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := openMirror(cmd)
			if err != nil {
				return err
			}
			defer m.Close()
			cands, err := m.ResolveWork(name, limit)
			if err != nil {
				return err
			}
			if cands == nil {
				cands = []catalog.WorkCandidate{}
			}
			return emitJSON(cmd, map[string]any{"candidates": cands})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "work name (required)")
	cmd.Flags().IntVar(&limit, "limit", 10, "max candidates")
	cmd.MarkFlagRequired("name")
	return cmd
}
