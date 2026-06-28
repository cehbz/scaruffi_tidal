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
}

var dcStmts = []string{
	// Minimal: a marker so ATTACH has something to read. Discogs query tables
	// are added in slice 2b.
	`CREATE TABLE dc_marker (ok INTEGER)`,
	`INSERT INTO dc_marker (ok) VALUES (1)`,
}
