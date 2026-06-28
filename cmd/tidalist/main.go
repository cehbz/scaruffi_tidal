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

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, `{"error":%q}`+"\n", err.Error())
		os.Exit(1)
	}
}
