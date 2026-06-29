// Package catalog reads the local MusicBrainz + Discogs SQLite mirrors and returns
// ranked candidates for the CLI tools. It opens MB read-only and ATTACHes Discogs
// as `dc`, porting src/tidalist/metadata/mirror.py.
package catalog

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"strings"

	sqlite "modernc.org/sqlite"
)

// MirrorDB is a read-only handle over the MusicBrainz mirror with the Discogs
// mirror attached as `dc`.
type MirrorDB struct {
	DB *sql.DB
}

func roURI(path string) string {
	return "file:" + url.PathEscape(path) + "?mode=ro&immutable=1"
}

// attachConnector opens the MusicBrainz mirror and runs `ATTACH … AS dc` on every
// new pooled connection. Because the attach is per-connection rather than a single
// pinned connection, the pool may grow freely: every connection it hands out sees
// `dc`, so a sub-query issued while a rows cursor from another connection is open
// simply draws a second (also-attached) connection instead of deadlocking.
//
// It is constructed only inside Open, so the plain sql.Open("sqlite", …) handles
// used elsewhere (e.g. internal/mirrorfixture seeding) are unaffected and never
// ATTACH.
type attachConnector struct {
	driver *sqlite.Driver
	mbURI  string
	dcURI  string
}

// Connect opens the MB connection and attaches the Discogs mirror as `dc`.
func (c *attachConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.driver.Open(c.mbURI)
	if err != nil {
		return nil, err
	}
	if _, err := conn.(driver.ExecerContext).ExecContext(ctx, "ATTACH DATABASE '"+c.dcURI+"' AS dc", nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("attach discogs: %w", err)
	}
	return conn, nil
}

// Driver returns the underlying modernc SQLite driver.
func (c *attachConnector) Driver() driver.Driver { return c.driver }

// Open opens the MB mirror read-only and ATTACHes the Discogs mirror as `dc` on
// every connection the pool creates.
func Open(mbPath, dcPath string) (*MirrorDB, error) {
	connector := &attachConnector{
		driver: &sqlite.Driver{},
		mbURI:  roURI(mbPath),
		dcURI:  roURI(dcPath),
	}
	db := sql.OpenDB(connector)
	// Open a connection eagerly so a bad path (or a failed ATTACH) fails here
	// rather than on the first query.
	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, err
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
