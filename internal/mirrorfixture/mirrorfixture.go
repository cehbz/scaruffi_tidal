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
	`CREATE TABLE release (id INTEGER PRIMARY KEY, gid TEXT, name TEXT, artist_credit INTEGER, release_group INTEGER, status INTEGER, discogs_release_id INTEGER)`,
	`CREATE TABLE medium (id INTEGER PRIMARY KEY, release INTEGER, position INTEGER, format INTEGER, track_count INTEGER)`,
	`CREATE TABLE track (id INTEGER PRIMARY KEY, gid TEXT, recording INTEGER, medium INTEGER, position INTEGER, number TEXT, name TEXT, length INTEGER)`,
	`CREATE TABLE canonical_musicbrainz_data (id INTEGER PRIMARY KEY, recording_mbid TEXT, release_mbid TEXT)`,
	`INSERT INTO release_group_secondary_type (id, name) VALUES (1, 'Compilation'), (2, 'Live')`,
	// JBMD — a studio album by Traffic (artist 1), with a Discogs master link and a canonical tracklist.
	`INSERT INTO release_group (id, gid, name, artist_credit, discogs_master_id) VALUES (60, 'rg-jbmd', 'John Barleycorn Must Die', 1, 69017)`,
	`INSERT INTO release_group_fts (rowid, title) VALUES (60, 'John Barleycorn Must Die')`,
	`INSERT INTO release_group_meta (id, first_release_date_year) VALUES (60, 1970)`,
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (20, 'r-glad', 'Glad', 419426, '', 1)`,
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (21, 'r-fr', 'Freedom Rider', 329266, '', 1)`,
	`INSERT INTO release (id, gid, name, artist_credit, release_group, status, discogs_release_id) VALUES (500, 'rel-jbmd', 'John Barleycorn Must Die', 1, 60, 1, 2087351)`,
	`INSERT INTO medium (id, release, position, format, track_count) VALUES (700, 500, 1, 1, 2)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES (800, 't-glad', 20, 700, 1, '1', 'Glad', 419093)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES (801, 't-fr', 21, 700, 2, '2', 'Freedom Rider', 329266)`,
	`INSERT INTO canonical_musicbrainz_data (id, recording_mbid, release_mbid) VALUES (1, 'r-glad', 'rel-jbmd'), (2, 'r-fr', 'rel-jbmd')`,
	`INSERT INTO isrc (recording, isrc) VALUES (20, 'GBUM71030667')`,
	// A compilation RG by Traffic, to exercise trait extraction.
	`INSERT INTO release_group (id, gid, name, artist_credit, discogs_master_id) VALUES (61, 'rg-best', 'Best of Traffic', 1, 0)`,
	`INSERT INTO release_group_fts (rowid, title) VALUES (61, 'Best of Traffic')`,
	`INSERT INTO release_group_meta (id, first_release_date_year) VALUES (61, 1975)`,
	`INSERT INTO release_group_secondary_type_join (release_group, secondary_type) VALUES (61, 1)`,
	// --- work + relationship links (classical) ---
	`CREATE TABLE work (id INTEGER PRIMARY KEY, gid TEXT, name TEXT, type INTEGER, comment TEXT)`,
	`CREATE VIRTUAL TABLE work_fts USING fts5(title, content='')`,
	`CREATE TABLE link (id INTEGER PRIMARY KEY, link_type INTEGER)`,
	`CREATE TABLE l_recording_work (id INTEGER PRIMARY KEY, link INTEGER, entity0 INTEGER, entity1 INTEGER, link_order INTEGER)`,
	`CREATE TABLE l_artist_work (id INTEGER PRIMARY KEY, link INTEGER, entity0 INTEGER, entity1 INTEGER, link_order INTEGER)`,
	`INSERT INTO link (id, link_type) VALUES (1, 278), (2, 168)`, // 278=performance, 168=composer
	`INSERT INTO work (id, gid, name, type, comment) VALUES (300, 'work-mpm', 'Missa Papae Marcelli', 1, '')`,
	`INSERT INTO work_fts (rowid, title) VALUES (300, 'Missa Papae Marcelli')`,
	`INSERT INTO artist (id, gid, name, comment) VALUES (3, 'a-palestrina', 'Palestrina', '')`,
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (30, 'r-kyrie', 'Kyrie', 360000, '', 1)`,
	`INSERT INTO l_recording_work (id, link, entity0, entity1, link_order) VALUES (1, 1, 30, 300, 0)`, // recording 30 performs work 300
	`INSERT INTO l_artist_work (id, link, entity0, entity1, link_order) VALUES (1, 2, 3, 300, 0)`,     // artist 3 (Palestrina) composes work 300
	`INSERT INTO isrc (recording, isrc) VALUES (30, 'GBCLASSICAL01')`,
	// --- edition tables (slice 2c) ---
	`CREATE TABLE release_country (release INTEGER, country INTEGER, date_year INTEGER, date_month INTEGER, date_day INTEGER)`,
	`CREATE TABLE release_unknown_country (release INTEGER PRIMARY KEY, date_year INTEGER, date_month INTEGER, date_day INTEGER)`,
	`CREATE TABLE area (id INTEGER PRIMARY KEY, gid TEXT, name TEXT)`,
	`CREATE TABLE medium_format (id INTEGER PRIMARY KEY, name TEXT)`,
	`CREATE TABLE release_label (id INTEGER PRIMARY KEY, release INTEGER, label INTEGER, catalog_number TEXT)`,
	`CREATE TABLE label (id INTEGER PRIMARY KEY, gid TEXT, name TEXT)`,
	`INSERT INTO area (id, gid, name) VALUES (1, 'area-gb', 'United Kingdom'), (2, 'area-us', 'United States')`,
	`INSERT INTO medium_format (id, name) VALUES (1, 'Vinyl'), (2, 'CD')`,
	`INSERT INTO label (id, gid, name) VALUES (1, 'label-island', 'Island Records'), (2, 'label-ua', 'United Artists')`,
	`INSERT INTO release_country (release, country, date_year, date_month, date_day) VALUES (500, 1, 1970, 7, NULL)`,
	`INSERT INTO release_label (id, release, label, catalog_number) VALUES (1, 500, 1, 'ILPS 9116')`,
	// a second US edition of the same release-group
	`INSERT INTO release (id, gid, name, artist_credit, release_group, status, discogs_release_id) VALUES (501, 'rel-jbmd-us', 'John Barleycorn Must Die', 1, 60, 1, NULL)`,
	`INSERT INTO medium (id, release, position, format, track_count) VALUES (701, 501, 1, 1, 2)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES (810, 't-glad-us', 20, 701, 1, '1', 'Glad', 419000)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES (811, 't-fr-us', 21, 701, 2, '2', 'Freedom Rider', 329000)`,
	`INSERT INTO release_country (release, country, date_year, date_month, date_day) VALUES (501, 2, 1970, NULL, NULL)`,
	`INSERT INTO release_label (id, release, label, catalog_number) VALUES (2, 501, 2, 'UAS 5504')`,
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
	`CREATE TABLE release (id INTEGER PRIMARY KEY, master_id INTEGER, is_main_release INTEGER, title TEXT, country TEXT, released_raw TEXT)`,
	`CREATE TABLE track (id INTEGER PRIMARY KEY, release_id INTEGER, parent_track_id INTEGER, seq INTEGER, position TEXT, title TEXT, duration TEXT)`,
	`INSERT INTO master (id, main_release_id, title, year) VALUES (69017, 583800, 'John Barleycorn Must Die', 1970)`,
	`INSERT INTO master_fts (rowid, title, artist_names) VALUES (69017, 'John Barleycorn Must Die', 'Traffic')`,
	`INSERT INTO master_artist (master_id, seq, artist_id) VALUES (69017, 1, 900)`,
	`INSERT INTO artist (id, name) VALUES (900, 'Traffic')`,
	`INSERT INTO master_genre (master_id, seq, genre) VALUES (69017, 1, 'Rock')`,
	`INSERT INTO master_style (master_id, seq, style) VALUES (69017, 1, 'Folk Rock'), (69017, 2, 'Blues Rock')`,
	`INSERT INTO release (id, master_id, is_main_release, title, country, released_raw) VALUES (583800, 69017, 1, 'John Barleycorn Must Die', 'UK', '1970-07-01')`,
	`INSERT INTO release (id, master_id, is_main_release, title, country, released_raw) VALUES (382820, 69017, 0, 'John Barleycorn Must Die', 'UK', '1987')`,
	`INSERT INTO track (id, release_id, parent_track_id, seq, position, title, duration) VALUES (100, 583800, NULL, 1, 'A1', 'Glad', '6:32'), (101, 583800, NULL, 2, 'A2', 'Freedom Rider', '6:20')`,
	// Reissue 382820: one top-level track plus a sub-track of it. The sub-track
	// (parent_track_id = 110) must be excluded by the parent_track_id IS NULL filter,
	// so the reissue's edition track_count is 1, not 2.
	`INSERT INTO track (id, release_id, parent_track_id, seq, position, title, duration) VALUES (110, 382820, NULL, 1, 'A', 'John Barleycorn Suite', '12:00')`,
	`INSERT INTO track (id, release_id, parent_track_id, seq, position, title, duration) VALUES (111, 382820, 110, 2, 'A.i', 'Part One', '6:00')`,
	`CREATE TABLE release_format (id INTEGER PRIMARY KEY, release_id INTEGER, seq INTEGER, name TEXT)`,
	`CREATE TABLE release_label (release_id INTEGER, seq INTEGER, label_id INTEGER, name TEXT, catno TEXT)`,
	`INSERT INTO release_format (id, release_id, seq, name) VALUES (1, 583800, 1, 'Vinyl'), (2, 382820, 1, 'Vinyl')`,
	// 382820's label name contains a comma to exercise comma-safe label extraction
	// (a GROUP_CONCAT split on "," would fragment it).
	`INSERT INTO release_label (release_id, seq, label_id, name, catno) VALUES (583800, 1, 1, 'Island Records', 'ILPS 9116'), (382820, 1, 2, 'PolyGram Records, Inc.', 'IRSP 10')`,
}
