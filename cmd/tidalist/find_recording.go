package main

import (
	"fmt"

	"github.com/cehbz/tidalist/catalog"
	"github.com/cehbz/tidalist/core"
	"github.com/spf13/cobra"
)

func newFindRecordingCmd() *cobra.Command {
	var title, artistMBID, isrc, work string
	var creditSpecs []string
	var limit int
	cmd := &cobra.Command{
		Use:   "find-recording",
		Short: "Find ranked recording candidates",
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" && work == "" {
				return fmt.Errorf("provide --title or --work")
			}
			credits, err := parseCredits(creditSpecs)
			if err != nil {
				return err
			}
			q := catalog.RecordingQuery{
				Title:      title,
				ArtistMBID: core.MBID(artistMBID),
				ISRC:       core.ISRC(isrc),
				Work:       work,
				Limit:      limit,
				Credits:    credits,
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
			res, err := m.FindRecording(q)
			if err != nil {
				return err
			}
			cands := res.Candidates
			if cands == nil {
				cands = []catalog.RecordingCandidate{}
			}
			out := map[string]any{"candidates": cands}
			if len(res.Warnings) > 0 {
				out["warnings"] = res.Warnings
			}
			if res.WorkResolution != "" {
				out["work_resolution"] = res.WorkResolution
			}
			return emitJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "recording title")
	cmd.Flags().StringVar(&work, "work", "", "work (composition) name")
	cmd.Flags().StringArrayVar(&creditSpecs, "credit", nil, "role:name[:attrs] (repeatable)")
	cmd.Flags().StringVar(&artistMBID, "artist-mbid", "", "artist MBID hint")
	cmd.Flags().StringVar(&isrc, "isrc", "", "ISRC to mark exact matches")
	cmd.Flags().IntVar(&limit, "limit", 10, "max candidates")
	return cmd
}
