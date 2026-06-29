package main

import (
	"github.com/cehbz/tidalist/catalog"
	"github.com/cehbz/tidalist/core"
	"github.com/spf13/cobra"
)

func newFindAlbumCmd() *cobra.Command {
	var title, artistMBID string
	var creditSpecs []string
	var year, limit int
	cmd := &cobra.Command{
		Use:   "find-album",
		Short: "Find ranked album candidates (MusicBrainz + Discogs peers)",
		RunE: func(cmd *cobra.Command, args []string) error {
			credits, err := parseCredits(creditSpecs)
			if err != nil {
				return err
			}
			q := catalog.AlbumQuery{
				Title:      title,
				ArtistMBID: core.MBID(artistMBID),
				Year:       year,
				Limit:      limit,
			}
			if names := credits.Names(core.RoleArtist); len(names) > 0 {
				q.ArtistName = names[0]
			}
			m, err := openMirror(cmd)
			if err != nil {
				return err
			}
			defer m.Close()
			cands, err := m.FindAlbum(q)
			if err != nil {
				return err
			}
			if cands == nil {
				cands = []catalog.AlbumCandidate{}
			}
			return emitJSON(cmd, map[string]any{"candidates": cands})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "album title (required)")
	cmd.Flags().StringArrayVar(&creditSpecs, "credit", nil, "role:name[:attrs] (repeatable)")
	cmd.Flags().StringVar(&artistMBID, "artist-mbid", "", "artist MBID hint")
	cmd.Flags().IntVar(&year, "year", 0, "release year hint")
	cmd.Flags().IntVar(&limit, "limit", 10, "max candidates per source")
	cmd.MarkFlagRequired("title")
	return cmd
}
