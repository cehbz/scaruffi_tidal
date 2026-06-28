package main

import (
	"github.com/cehbz/tidalist/catalog"
	"github.com/spf13/cobra"
)

func newResolveArtistCmd() *cobra.Command {
	var name string
	var limit int
	cmd := &cobra.Command{
		Use:   "resolve-artist",
		Short: "Resolve an artist name to ranked MusicBrainz candidates",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := openMirror(cmd)
			if err != nil {
				return err
			}
			defer m.Close()
			cands, err := m.ResolveArtist(name, limit)
			if err != nil {
				return err
			}
			if cands == nil {
				cands = []catalog.ArtistCandidate{}
			}
			return emitJSON(cmd, map[string]any{"candidates": cands})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "artist name (required)")
	cmd.Flags().IntVar(&limit, "limit", 10, "max candidates")
	cmd.MarkFlagRequired("name")
	return cmd
}
