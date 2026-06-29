package main

import (
	"fmt"

	"github.com/cehbz/tidalist/catalog"
	"github.com/spf13/cobra"
)

func newTracklistCmd() *cobra.Command {
	var rg string
	var master int64
	cmd := &cobra.Command{
		Use:   "tracklist",
		Short: "Canonical tracklist for an album (--rg MB release-group, or --master Discogs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (rg == "") == (master == 0) {
				return fmt.Errorf("provide exactly one of --rg or --master")
			}
			m, err := openMirror(cmd)
			if err != nil {
				return err
			}
			defer m.Close()
			var tracks []catalog.Track
			if rg != "" {
				tracks, err = m.TracklistByReleaseGroup(rg)
			} else {
				tracks, err = m.TracklistByMaster(master)
			}
			if err != nil {
				return err
			}
			if tracks == nil {
				tracks = []catalog.Track{}
			}
			return emitJSON(cmd, map[string]any{"tracks": tracks})
		},
	}
	cmd.Flags().StringVar(&rg, "rg", "", "MusicBrainz release-group MBID")
	cmd.Flags().Int64Var(&master, "master", 0, "Discogs master id")
	return cmd
}
