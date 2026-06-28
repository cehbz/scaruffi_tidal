package main

import (
	"github.com/cehbz/tidalist/catalog"
	"github.com/cehbz/tidalist/core"
	"github.com/spf13/cobra"
)

func newFindRecordingCmd() *cobra.Command {
	var title, artistMBID, isrc string
	var creditSpecs []string
	var limit int
	cmd := &cobra.Command{
		Use:   "find-recording",
		Short: "Find ranked recording candidates",
		RunE: func(cmd *cobra.Command, args []string) error {
			credits, err := parseCredits(creditSpecs)
			if err != nil {
				return err
			}
			q := catalog.RecordingQuery{
				Title:      title,
				ArtistMBID: core.MBID(artistMBID),
				ISRC:       core.ISRC(isrc),
				Limit:      limit,
			}
			if q.ArtistMBID == "" {
				if names := credits.Names(core.RoleArtist); len(names) > 0 {
					q.ArtistName = names[0]
				}
			}
			m, err := openMirror(cmd)
			if err != nil {
				return err
			}
			defer m.Close()
			cands, err := m.FindRecording(q)
			if err != nil {
				return err
			}
			if cands == nil {
				cands = []catalog.RecordingCandidate{}
			}
			return emitJSON(cmd, map[string]any{"candidates": cands})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "recording title (required)")
	cmd.Flags().StringArrayVar(&creditSpecs, "credit", nil, "role:name[:attrs] (repeatable)")
	cmd.Flags().StringVar(&artistMBID, "artist-mbid", "", "artist MBID hint")
	cmd.Flags().StringVar(&isrc, "isrc", "", "ISRC to mark exact matches")
	cmd.Flags().IntVar(&limit, "limit", 10, "max candidates")
	cmd.MarkFlagRequired("title")
	return cmd
}
