package main

import (
	"fmt"

	"github.com/cehbz/tidalist/catalog"
	"github.com/spf13/cobra"
)

func newAlbumEditionsCmd() *cobra.Command {
	var rg string
	var master int64
	cmd := &cobra.Command{
		Use:   "album-editions",
		Short: "List an album's editions (--rg MB release-group, or --master Discogs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (rg == "") == (master == 0) {
				return fmt.Errorf("provide exactly one of --rg or --master")
			}
			m, err := openMirror(cmd)
			if err != nil {
				return err
			}
			defer m.Close()
			var eds []catalog.Edition
			if rg != "" {
				eds, err = m.AlbumEditionsMB(rg)
			} else {
				eds, err = m.AlbumEditionsDiscogs(master)
			}
			if err != nil {
				return err
			}
			if eds == nil {
				eds = []catalog.Edition{}
			}
			return emitJSON(cmd, map[string]any{"editions": eds})
		},
	}
	cmd.Flags().StringVar(&rg, "rg", "", "MusicBrainz release-group MBID")
	cmd.Flags().Int64Var(&master, "master", 0, "Discogs master id")
	return cmd
}
