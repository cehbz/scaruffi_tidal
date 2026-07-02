package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cehbz/tidalist/catalog"
	"github.com/spf13/cobra"
)

const (
	defaultMBDB = "/Volumes/Crucial X10/musicbrainz/musicbrainz.db"
	defaultDCDB = "/Volumes/Crucial X10/discogs/discogs.db"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "tidalist", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("musicbrainz-db", envOr("TIDALIST_MUSICBRAINZ_DB", defaultMBDB), "MusicBrainz mirror path")
	root.PersistentFlags().String("discogs-db", envOr("TIDALIST_DISCOGS_DB", defaultDCDB), "Discogs mirror path")
	root.AddCommand(newResolveArtistCmd())
	root.AddCommand(newResolveWorkCmd())
	root.AddCommand(newResolvePerformanceCmd())
	root.AddCommand(newFindRecordingCmd())
	root.AddCommand(newFindAlbumCmd())
	root.AddCommand(newTracklistCmd())
	root.AddCommand(newAlbumEditionsCmd())
	root.AddCommand(newLintIntentCmd())
	root.AddCommand(newMaterializeGoldenCmd())
	return root
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func openMirror(cmd *cobra.Command) (*catalog.MirrorDB, error) {
	mb, _ := cmd.Flags().GetString("musicbrainz-db")
	dc, _ := cmd.Flags().GetString("discogs-db")
	return catalog.Open(mb, dc)
}

// emitJSON writes v as compact JSON to the command's stdout.
func emitJSON(cmd *cobra.Command, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}

// errorJSON renders err as a one-line JSON object {"error":"…"} (valid for any input).
func errorJSON(err error) string {
	b, e := json.Marshal(map[string]string{"error": err.Error()})
	if e != nil {
		return `{"error":"internal error"}`
	}
	return string(b)
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, errorJSON(err))
		os.Exit(1)
	}
}
