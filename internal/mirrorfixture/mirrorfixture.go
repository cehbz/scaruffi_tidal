// Package mirrorfixture builds a minimal temp-dir SQLite fixture matching the
// subset of the MusicBrainz + Discogs mirror schema the catalog queries touch.
// It is module-internal test support shared by the catalog and cmd test packages
// (Go test helpers are not cross-importable), so it lives in a regular package and
// returns errors rather than importing testing.
package mirrorfixture

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Build creates mb.db and dc.db under dir, seeds them, and returns their paths.
func Build(dir string) (mbPath, dcPath string, err error) {
	mbPath = filepath.Join(dir, "mb.db")
	dcPath = filepath.Join(dir, "dc.db")
	if err = exec(mbPath, mbStmts); err != nil {
		return "", "", err
	}
	if err = exec(dcPath, dcStmts); err != nil {
		return "", "", err
	}
	return mbPath, dcPath, nil
}

func exec(path string, stmts []string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("seed %q: %w", s, err)
		}
	}
	return nil
}

var mbStmts = []string{
	`CREATE TABLE artist (id INTEGER PRIMARY KEY, gid TEXT, name TEXT, comment TEXT)`,
	`CREATE VIRTUAL TABLE artist_fts USING fts5(name, content='')`,
	`CREATE TABLE artist_credit_name (artist_credit INTEGER, artist INTEGER)`,
	`CREATE TABLE recording (id INTEGER PRIMARY KEY, gid TEXT, name TEXT, length INTEGER, comment TEXT, artist_credit INTEGER)`,
	`CREATE VIRTUAL TABLE recording_fts USING fts5(title, content='')`,
	`CREATE TABLE isrc (recording INTEGER, isrc TEXT)`,
	`INSERT INTO artist (id, gid, name, comment) VALUES (1, 'a-traffic', 'Traffic', '')`,
	`INSERT INTO artist (id, gid, name, comment) VALUES (2, 'a-traffic-sound', 'Traffic Sound', 'Peruvian band')`,
	`INSERT INTO artist_fts (rowid, name) VALUES (1, 'Traffic'), (2, 'Traffic Sound')`,
	`INSERT INTO artist_credit_name (artist_credit, artist) VALUES (1, 1)`,
	`INSERT INTO artist_credit_name (artist_credit, artist) VALUES (99, 2)`,
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (10, 'r-dmf', 'Dear Mr. Fantasy', 300000, '', 1)`,
	`INSERT INTO recording_fts (rowid, title) VALUES (10, 'Dear Mr. Fantasy')`,
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (11, 'r-dmf-cover', 'Dear Mr. Fantasy', 290000, '', 99)`,
	`INSERT INTO recording_fts (rowid, title) VALUES (11, 'Dear Mr. Fantasy')`,
	`INSERT INTO isrc (recording, isrc) VALUES (10, 'GBABC1234567')`,
	// --- album-side (release-group, release, medium, track, canonical, secondary types) ---
	`CREATE TABLE release_group (id INTEGER PRIMARY KEY, gid TEXT, name TEXT, artist_credit INTEGER, discogs_master_id INTEGER)`,
	`CREATE VIRTUAL TABLE release_group_fts USING fts5(title, content='')`,
	`CREATE TABLE release_group_meta (id INTEGER PRIMARY KEY, first_release_date_year INTEGER)`,
	`CREATE TABLE release_group_secondary_type (id INTEGER PRIMARY KEY, name TEXT)`,
	`CREATE TABLE release_group_secondary_type_join (release_group INTEGER, secondary_type INTEGER)`,
	`CREATE TABLE release (id INTEGER PRIMARY KEY, gid TEXT, name TEXT, artist_credit INTEGER, release_group INTEGER)`,
	`CREATE TABLE medium (id INTEGER PRIMARY KEY, release INTEGER, position INTEGER, track_count INTEGER)`,
	`CREATE TABLE track (id INTEGER PRIMARY KEY, gid TEXT, recording INTEGER, medium INTEGER, position INTEGER, number TEXT, name TEXT, length INTEGER)`,
	`CREATE TABLE canonical_musicbrainz_data (id INTEGER PRIMARY KEY, recording_mbid TEXT, release_mbid TEXT)`,
	`INSERT INTO release_group_secondary_type (id, name) VALUES (1, 'Compilation'), (2, 'Live')`,
	// JBMD — a studio album by Traffic (artist 1), with a Discogs master link and a canonical tracklist.
	`INSERT INTO release_group (id, gid, name, artist_credit, discogs_master_id) VALUES (60, 'rg-jbmd', 'John Barleycorn Must Die', 1, 69017)`,
	`INSERT INTO release_group_fts (rowid, title) VALUES (60, 'John Barleycorn Must Die')`,
	`INSERT INTO release_group_meta (id, first_release_date_year) VALUES (60, 1970)`,
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (20, 'r-glad', 'Glad', 419426, '', 1)`,
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (21, 'r-fr', 'Freedom Rider', 329266, '', 1)`,
	`INSERT INTO release (id, gid, name, artist_credit, release_group) VALUES (500, 'rel-jbmd', 'John Barleycorn Must Die', 1, 60)`,
	`INSERT INTO medium (id, release, position, track_count) VALUES (700, 500, 1, 2)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES (800, 't-glad', 20, 700, 1, '1', 'Glad', 419093)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES (801, 't-fr', 21, 700, 2, '2', 'Freedom Rider', 329266)`,
	`INSERT INTO canonical_musicbrainz_data (id, recording_mbid, release_mbid) VALUES (1, 'r-glad', 'rel-jbmd'), (2, 'r-fr', 'rel-jbmd')`,
	`INSERT INTO isrc (recording, isrc) VALUES (20, 'GBUM71030667')`,
	// A compilation RG by Traffic, to exercise trait extraction.
	`INSERT INTO release_group (id, gid, name, artist_credit, discogs_master_id) VALUES (61, 'rg-best', 'Best of Traffic', 1, 0)`,
	`INSERT INTO release_group_fts (rowid, title) VALUES (61, 'Best of Traffic')`,
	`INSERT INTO release_group_meta (id, first_release_date_year) VALUES (61, 1975)`,
	`INSERT INTO release_group_secondary_type_join (release_group, secondary_type) VALUES (61, 1)`,
}

var dcStmts = []string{
	// Minimal: a marker so ATTACH has something to read. Discogs query tables
	// are added in slice 2b.
	`CREATE TABLE dc_marker (ok INTEGER)`,
	`INSERT INTO dc_marker (ok) VALUES (1)`,
	`CREATE TABLE master (id INTEGER PRIMARY KEY, main_release_id INTEGER, title TEXT, year INTEGER)`,
	`CREATE VIRTUAL TABLE master_fts USING fts5(title, artist_names, content='')`,
	`CREATE TABLE master_artist (master_id INTEGER, seq INTEGER, artist_id INTEGER)`,
	`CREATE TABLE artist (id INTEGER PRIMARY KEY, name TEXT)`,
	`CREATE TABLE master_genre (master_id INTEGER, seq INTEGER, genre TEXT)`,
	`CREATE TABLE master_style (master_id INTEGER, seq INTEGER, style TEXT)`,
	`CREATE TABLE release (id INTEGER PRIMARY KEY, master_id INTEGER, is_main_release INTEGER)`,
	`CREATE TABLE track (release_id INTEGER, seq INTEGER, position TEXT, title TEXT, duration TEXT)`,
	`INSERT INTO master (id, main_release_id, title, year) VALUES (69017, 583800, 'John Barleycorn Must Die', 1970)`,
	`INSERT INTO master_fts (rowid, title, artist_names) VALUES (69017, 'John Barleycorn Must Die', 'Traffic')`,
	`INSERT INTO master_artist (master_id, seq, artist_id) VALUES (69017, 1, 900)`,
	`INSERT INTO artist (id, name) VALUES (900, 'Traffic')`,
	`INSERT INTO master_genre (master_id, seq, genre) VALUES (69017, 1, 'Rock')`,
	`INSERT INTO master_style (master_id, seq, style) VALUES (69017, 1, 'Folk Rock'), (69017, 2, 'Blues Rock')`,
	`INSERT INTO release (id, master_id, is_main_release) VALUES (583800, 69017, 1)`,
	`INSERT INTO track (release_id, seq, position, title, duration) VALUES (583800, 1, 'A1', 'Glad', '6:32'), (583800, 2, 'A2', 'Freedom Rider', '6:20')`,
}
