// Package catalog reads the local MusicBrainz + Discogs SQLite mirrors and returns
// ranked candidates for the CLI tools. It opens MB read-only and ATTACHes Discogs
// as `dc`, porting src/tidalist/metadata/mirror.py.
package catalog

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	_ "modernc.org/sqlite"
)

// MirrorDB is a read-only handle over the MusicBrainz mirror with the Discogs
// mirror attached as `dc`.
type MirrorDB struct {
	DB *sql.DB
}

func roURI(path string) string {
	return "file:" + url.PathEscape(path) + "?mode=ro&immutable=1"
}

// Open opens the MB mirror read-only and ATTACHes the Discogs mirror as `dc`.
func Open(mbPath, dcPath string) (*MirrorDB, error) {
	db, err := sql.Open("sqlite", roURI(mbPath))
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS dc", roURI(dcPath))); err != nil {
		db.Close()
		return nil, fmt.Errorf("attach discogs: %w", err)
	}
	return &MirrorDB{DB: db}, nil
}

func (m *MirrorDB) Close() error { return m.DB.Close() }

// escapeFTS wraps a term in double quotes, doubling embedded quotes, for a safe
// FTS5 MATCH phrase.
func escapeFTS(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// ftsTitle builds a title-column FTS phrase: title:"…".
func ftsTitle(s string) string { return "title:" + escapeFTS(s) }
