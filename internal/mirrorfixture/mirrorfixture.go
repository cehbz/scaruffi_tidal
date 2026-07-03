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
	all := append(append([]string{}, mbStmts...), mbTwinFamilyStmts...)
	all = append(all, mbAliasStmts...)
	all = append(all, mbEnsembleAliasStmts...)
	all = append(all, mbWorkAliasStmts...)
	all = append(all, mbGenericTitleFloodStmts()...)
	if err = exec(mbPath, all); err != nil {
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
	`CREATE TABLE artist (id INTEGER PRIMARY KEY, gid TEXT, name TEXT, comment TEXT, type INTEGER, discogs_artist_id INTEGER)`,
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
	`CREATE TABLE l_work_work (id INTEGER PRIMARY KEY, link INTEGER, entity0 INTEGER, entity1 INTEGER, link_order INTEGER)`,
	`INSERT INTO link (id, link_type) VALUES (1, 278), (2, 168)`, // 278=performance, 168=composer
	`INSERT INTO work (id, gid, name, type, comment) VALUES (300, 'work-mpm', 'Missa Papae Marcelli', 1, '')`,
	`INSERT INTO work_fts (rowid, title) VALUES (300, 'Missa Papae Marcelli')`,
	`INSERT INTO artist (id, gid, name, comment) VALUES (3, 'a-palestrina', 'Palestrina', '')`,
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (30, 'r-kyrie', 'Kyrie', 360000, '', 1)`,
	`INSERT INTO recording_fts (rowid, title) VALUES (30, 'Kyrie')`,                                   // FTS-searchable + credit-bearing → exercises the title-path --credit filter
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
	// --- recording↔artist performer relationships (slice 2d) ---
	`CREATE TABLE link_type (id INTEGER PRIMARY KEY, name TEXT)`,
	`CREATE TABLE l_artist_recording (id INTEGER PRIMARY KEY, link INTEGER, entity0 INTEGER, entity1 INTEGER, link_order INTEGER, entity0_credit TEXT)`,
	`CREATE TABLE link_attribute (link INTEGER, attribute_type INTEGER)`,
	`CREATE TABLE link_attribute_type (id INTEGER PRIMARY KEY, root INTEGER, name TEXT)`,
	`INSERT INTO link_type (id, name) VALUES (156,'performer'),(148,'instrument'),(149,'vocal'),(150,'performing orchestra'),(151,'conductor'),(152,'chorus master')`,
	`INSERT INTO link_attribute_type (id, root, name) VALUES (14,14,'instrument'),(3,3,'vocal'),(180,14,'piano'),(10,3,'soprano vocals'),(13,3,'choir vocals')`,
	// performer artists (type: 1=Person,5=Orchestra,6=Choir)
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (40,'a-tallis','The Tallis Scholars','',5)`,
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (41,'a-phillips','Peter Phillips','',1)`,
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (42,'a-kirkby','Emma Kirkby','',1)`,
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (43,'a-choir','Oxford Choir','',6)`,
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (44,'a-chorusmaster','Some Chorusmaster','',1)`,
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (45,'a-bsmith','Barnaby Smith','',1)`,
	// link rows (link.id -> link_type); 2c already inserted link (1,278),(2,168)
	`INSERT INTO link (id, link_type) VALUES (10,150),(11,151),(12,148),(13,149),(14,152),(15,150),(16,152)`,
	// recording 30 (r-kyrie, work 300): orchestra(Tallis), conductor(Phillips), soloist(Kirkby,piano), chorus(Oxford Choir via choir vocals), AND chorus-master(surfaced as chorus_master)
	`INSERT INTO l_artist_recording (id, link, entity0, entity1, link_order, entity0_credit) VALUES (1,10,40,30,0,''),(2,11,41,30,0,''),(3,12,42,30,0,''),(4,13,43,30,0,''),(5,14,44,30,0,'')`,
	`INSERT INTO link_attribute (link, attribute_type) VALUES (12,180),(13,13)`, // soloist Kirkby->piano(180); choir vocal->choir vocals(13)
	// recording 31: a different orchestra (artist 1 Traffic), for the orchestra --credit filter
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (31,'r-mpm-other','Missa Papae Marcelli',360000,'',1)`,
	`INSERT INTO l_recording_work (id, link, entity0, entity1, link_order) VALUES (2,1,31,300,0)`,
	`INSERT INTO l_artist_recording (id, link, entity0, entity1, link_order, entity0_credit) VALUES (6,15,1,31,0,'')`,
	`INSERT INTO isrc (recording, isrc) VALUES (31,'GBCLASSICAL02')`,
	// recording 32: a STANDALONE chorus-master (Barnaby Smith via 152, NO conductor) — the de-facto director; reachable via the conductor umbrella
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES (32,'r-mpm-acap','Missa Papae Marcelli',360000,'',1)`,
	`INSERT INTO l_recording_work (id, link, entity0, entity1, link_order) VALUES (3,1,32,300,0)`,
	`INSERT INTO l_artist_recording (id, link, entity0, entity1, link_order, entity0_credit) VALUES (7,16,45,32,0,'')`,
	// --- work-group (slice 2e): 281 parts, 350 arrangement (excluded) ---
	`INSERT INTO link (id, link_type) VALUES (20,281),(21,350)`,
	// Parent work + four movements (work-group via l_work_work 281 parts).
	`INSERT INTO artist (id, gid, name, comment, type, discogs_artist_id) VALUES (50,'a-beethoven','Ludwig van Beethoven','',1,952)`,
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (51,'a-brahms','Johannes Brahms','',1)`,
	// Mahler bridge (the multi-composer-trap decoy's actual composer).
	`INSERT INTO artist (id, gid, name, comment, type, discogs_artist_id) VALUES (52,'a-mahler','Gustav Mahler','',1,953)`,
	`INSERT INTO work (id, gid, name, type, comment) VALUES (310,'w-sym5','Symphony no. 5 in C minor, op. 67',1,'')`,
	`INSERT INTO work (id, gid, name, type, comment) VALUES (311,'w-sym5-i','Symphony no. 5 in C minor, op. 67: I. Allegro con brio',1,'')`,
	`INSERT INTO work (id, gid, name, type, comment) VALUES (312,'w-sym5-ii','Symphony no. 5 in C minor, op. 67: II. Andante con moto',1,'')`,
	`INSERT INTO work (id, gid, name, type, comment) VALUES (313,'w-sym5-iii','Symphony no. 5 in C minor, op. 67: III. Scherzo',1,'')`,
	`INSERT INTO work (id, gid, name, type, comment) VALUES (314,'w-sym5-iv','Symphony no. 5 in C minor, op. 67: IV. Allegro',1,'')`,
	// A same-title work by a DIFFERENT composer (Brahms) — composer must disambiguate.
	`INSERT INTO work (id, gid, name, type, comment) VALUES (320,'w-brahms5','Symphony no. 5 in C minor',1,'')`,
	// A title-twin work with ZERO recordings (the arc-less stub FTS often ranks first).
	`INSERT INTO work (id, gid, name, type, comment) VALUES (321,'w-stub','Symphony no. 5 in C minor, op. 67',1,'')`,
	// An ARRANGEMENT of the parent (link_type 350) — must be excluded from the group.
	`INSERT INTO work (id, gid, name, type, comment) VALUES (322,'w-sym5-arr','Symphony no. 5 in C minor, op. 67 (arr. for piano)',1,'')`,
	`INSERT INTO work_fts (rowid, title) VALUES
		(310,'Symphony no. 5 in C minor, op. 67'),
		(311,'Symphony no. 5 in C minor, op. 67: I. Allegro con brio'),
		(312,'Symphony no. 5 in C minor, op. 67: II. Andante con moto'),
		(313,'Symphony no. 5 in C minor, op. 67: III. Scherzo'),
		(314,'Symphony no. 5 in C minor, op. 67: IV. Allegro'),
		(320,'Symphony no. 5 in C minor'),
		(321,'Symphony no. 5 in C minor, op. 67'),
		(322,'Symphony no. 5 in C minor, op. 67 (arr. for piano)')`,
	// composer arcs (l_artist_work 168): Beethoven composes the parent + movements; Brahms the same-title twin.
	`INSERT INTO l_artist_work (id, link, entity0, entity1, link_order) VALUES
		(10,2,50,310,0),(11,2,50,311,0),(12,2,50,312,0),(13,2,50,313,0),(14,2,50,314,0),
		(15,2,51,320,0),(16,2,50,321,0),(17,2,50,322,0)`,
	// work-group edges (l_work_work): 281 parts parent(310)->movements(311..314); 350 arrangement 310->322 (excluded).
	`INSERT INTO l_work_work (id, link, entity0, entity1, link_order) VALUES
		(1,20,310,311,1),(2,20,310,312,2),(3,20,310,313,3),(4,20,310,314,4),
		(5,21,310,322,0)`,
	// --- performances (Task 2): two co-release takes of the Beethoven symphony 5 work-group ---
	// performance forces (shared by both takes).
	`INSERT INTO artist (id, gid, name, comment, type, discogs_artist_id) VALUES (60,'a-bernstein','Leonard Bernstein','',1,299702)`,
	`INSERT INTO artist (id, gid, name, comment, type, discogs_artist_id) VALUES (61,'a-nyphil','New York Philharmonic','',5,950)`,
	`INSERT INTO artist_fts (rowid, name) VALUES (60,'Leonard Bernstein'),(61,'New York Philharmonic'),(50,'Ludwig van Beethoven'),(52,'Gustav Mahler')`,
	`INSERT INTO link (id, link_type) VALUES (30,151),(31,150)`, // 151 conductor, 150 orchestra
	// TAKE A (1963): movement recordings 40..43 on release-group 70 (rel 510, year 1963).
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES
		(40,'r-a-i','Symphony no. 5: I. Allegro con brio',450000,'',1),
		(41,'r-a-ii','Symphony no. 5: II. Andante con moto',600000,'',1),
		(42,'r-a-iii','Symphony no. 5: III. Scherzo',300000,'',1),
		(43,'r-a-iv','Symphony no. 5: IV. Allegro',519000,'',1)`,
	`INSERT INTO l_recording_work (id, link, entity0, entity1, link_order) VALUES
		(10,1,40,311,0),(11,1,41,312,0),(12,1,42,313,0),(13,1,43,314,0)`,
	`INSERT INTO l_artist_recording (id, link, entity0, entity1, link_order, entity0_credit) VALUES
		(20,30,60,40,0,''),(21,31,61,40,0,''),(22,30,60,41,0,''),(23,31,61,41,0,''),
		(24,30,60,42,0,''),(25,31,61,42,0,''),(26,30,60,43,0,''),(27,31,61,43,0,'')`,
	`INSERT INTO isrc (recording, isrc) VALUES (40,'USA196300001'),(41,'USA196300002'),(42,'USA196300003'),(43,'USA196300004')`,
	`INSERT INTO release_group (id, gid, name, artist_credit, discogs_master_id) VALUES (70,'rg-a','Beethoven: Symphony no. 5',1,70000)`,
	`INSERT INTO release_group_meta (id, first_release_date_year) VALUES (70,1963)`,
	`INSERT INTO release (id, gid, name, artist_credit, release_group, status, discogs_release_id) VALUES (510,'rel-a','Beethoven: Symphony no. 5',1,70,1,NULL)`,
	`INSERT INTO medium (id, release, position, format, track_count) VALUES (710,510,1,2,4)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES
		(820,'t-a-i',40,710,1,'1','I. Allegro con brio',450000),
		(821,'t-a-ii',41,710,2,'2','II. Andante con moto',600000),
		(822,'t-a-iii',42,710,3,'3','III. Scherzo',300000),
		(823,'t-a-iv',43,710,4,'4','IV. Allegro',519000)`,
	// canonical release pointer for TracklistByReleaseGroup(rg-a): each movement
	// recording's canonical release is rel-a, so the RG's canonical tracklist
	// resolves to all four movements (needed for ReleaseGroupCredits' performer-arc
	// aggregation over the canonical tracklist).
	`INSERT INTO canonical_musicbrainz_data (id, recording_mbid, release_mbid) VALUES
		(3,'r-a-i','rel-a'),(4,'r-a-ii','rel-a'),(5,'r-a-iii','rel-a'),(6,'r-a-iv','rel-a')`,
	// TAKE B (1985): SAME forces, movement recordings 44..47 on release-group 71 (rel 511, year 1985).
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES
		(44,'r-b-i','Symphony no. 5: I. Allegro con brio',455000,'',1),
		(45,'r-b-ii','Symphony no. 5: II. Andante con moto',610000,'',1),
		(46,'r-b-iii','Symphony no. 5: III. Scherzo',305000,'',1),
		(47,'r-b-iv','Symphony no. 5: IV. Allegro',418000,'',1)`,
	`INSERT INTO l_recording_work (id, link, entity0, entity1, link_order) VALUES
		(14,1,44,311,0),(15,1,45,312,0),(16,1,46,313,0),(17,1,47,314,0)`,
	`INSERT INTO l_artist_recording (id, link, entity0, entity1, link_order, entity0_credit) VALUES
		(28,30,60,44,0,''),(29,31,61,44,0,''),(30,30,60,45,0,''),(31,31,61,45,0,''),
		(32,30,60,46,0,''),(33,31,61,46,0,''),(34,30,60,47,0,''),(35,31,61,47,0,'')`,
	`INSERT INTO isrc (recording, isrc) VALUES (44,'USA198500001'),(45,'USA198500002'),(46,'USA198500003'),(47,'USA198500004')`,
	`INSERT INTO release_group (id, gid, name, artist_credit, discogs_master_id) VALUES (71,'rg-b','Beethoven: Symphony no. 5',1,70001)`,
	`INSERT INTO release_group_meta (id, first_release_date_year) VALUES (71,1985)`,
	`INSERT INTO release (id, gid, name, artist_credit, release_group, status, discogs_release_id) VALUES (511,'rel-b','Beethoven: Symphony no. 5',1,71,1,NULL)`,
	`INSERT INTO medium (id, release, position, format, track_count) VALUES (711,511,1,2,4)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES
		(830,'t-b-i',44,711,1,'1','I. Allegro con brio',455000),
		(831,'t-b-ii',45,711,2,'2','II. Andante con moto',610000),
		(832,'t-b-iii',46,711,3,'3','III. Scherzo',305000),
		(833,'t-b-iv',47,711,4,'4','IV. Allegro',418000)`,
}

var mbTwinFamilyStmts = []string{
	// --- title-twin work families (the Goldberg case): an English-named family whose
	// recordings are arrangements by OTHER performers, and a German-named family
	// ("Variationen" — never matched by the English FTS phrase) carrying the queried
	// performer's recordings. Work-group resolution by title lands on the English
	// family; the performer-driven fallback must find the German one.
	`INSERT INTO artist (id, gid, name, comment, type, discogs_artist_id) VALUES
		(62,'a-gould','Glenn Gould','',1,0),
		(63,'a-trio','Decoy String Trio','',5,0),
		(64,'a-bach','Johann Sebastian Bach','',1,0)`,
	`INSERT INTO artist_fts (rowid, name) VALUES (62,'Glenn Gould'),(63,'Decoy String Trio'),(64,'Johann Sebastian Bach')`,
	`INSERT INTO artist_credit_name (artist_credit, artist) VALUES (62,62),(63,63)`,
	// English family: parent 330 + child 331 (composer Bach on both).
	`INSERT INTO work (id, gid, name, type, comment) VALUES
		(330,'w-gv-en','Goldberg Variations',1,''),
		(331,'w-gv-en-v1','Goldberg Variations: Variation 1',1,'')`,
	`INSERT INTO work_fts (rowid, title) VALUES (330,'Goldberg Variations'),(331,'Goldberg Variations: Variation 1')`,
	`INSERT INTO l_artist_work (id, link, entity0, entity1, link_order) VALUES
		(30,2,64,330,0),(31,2,64,331,0)`,
	`INSERT INTO l_work_work (id, link, entity0, entity1, link_order) VALUES (10,20,330,331,1)`,
	// The English child's only recording is by the decoy trio.
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES
		(60,'r-gv-trio','Goldberg Variations: Variation 1 (string trio)',120000,'',63)`,
	`INSERT INTO l_recording_work (id, link, entity0, entity1, link_order) VALUES (30,1,60,331,0)`,
	// German family: parent 340 + children 341/342 (composer Bach), Gould's recordings.
	`INSERT INTO work (id, gid, name, type, comment) VALUES
		(340,'w-gv-de','Goldberg-Variationen, BWV 988',1,''),
		(341,'w-gv-de-v1','Goldberg-Variationen, BWV 988: Variatio 1',1,''),
		(342,'w-gv-de-v2','Goldberg-Variationen, BWV 988: Variatio 2',1,'')`,
	`INSERT INTO work_fts (rowid, title) VALUES
		(340,'Goldberg-Variationen, BWV 988'),
		(341,'Goldberg-Variationen, BWV 988: Variatio 1'),
		(342,'Goldberg-Variationen, BWV 988: Variatio 2')`,
	`INSERT INTO l_artist_work (id, link, entity0, entity1, link_order) VALUES (32,2,64,340,0)`,
	`INSERT INTO l_work_work (id, link, entity0, entity1, link_order) VALUES
		(11,20,340,341,1),(12,20,340,342,2)`,
	// Gould's two variation recordings (artist_credit 62 + instrument arc link 12=148).
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES
		(61,'r-gv-g1','Goldberg-Variationen: Variatio 1',115000,'',62),
		(62,'r-gv-g2','Goldberg-Variationen: Variatio 2',95000,'',62)`,
	`INSERT INTO l_recording_work (id, link, entity0, entity1, link_order) VALUES
		(31,1,61,341,0),(32,1,62,342,0)`,
	`INSERT INTO l_artist_recording (id, link, entity0, entity1, link_order, entity0_credit) VALUES
		(40,12,62,61,0,''),(41,12,62,62,0,'')`,
	`INSERT INTO isrc (recording, isrc) VALUES (61,'USG195500001'),(62,'USG195500002')`,
	// Both recordings first-co-release on release-group 72 (1955).
	`INSERT INTO release_group (id, gid, name, artist_credit, discogs_master_id) VALUES (72,'rg-gould-55','Goldberg Variations',62,0)`,
	`INSERT INTO release_group_meta (id, first_release_date_year) VALUES (72,1955)`,
	`INSERT INTO release (id, gid, name, artist_credit, release_group, status, discogs_release_id) VALUES (512,'rel-gould-55','Goldberg Variations',62,72,1,NULL)`,
	`INSERT INTO medium (id, release, position, format, track_count) VALUES (712,512,1,2,2)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES
		(840,'t-gv-1',61,712,1,'1','Variatio 1',115000),
		(841,'t-gv-2',62,712,2,'2','Variatio 2',95000)`,
}

var mbAliasStmts = []string{
	// --- Cyrillic-primary artists (the Gergiev/Stravinsky case): MB stores native-
	// script primary names with Latin forms in artist_alias. Name matching must
	// consult aliases or every Russian composer/performer zero-misses.
	`CREATE TABLE artist_alias (id INTEGER PRIMARY KEY, artist INTEGER, name TEXT, locale TEXT, type INTEGER)`,
	`INSERT INTO artist (id, gid, name, comment, type, discogs_artist_id) VALUES
		(65,'a-stravinsky','Игорь Фёдорович Стравинский','',1,0),
		(66,'a-gergiev','Валерий Гергиев','',1,0),
		(67,'a-kirov','Оркестр Мариинского театра','',5,0)`,
	`INSERT INTO artist_fts (rowid, name) VALUES
		(65,'Игорь Фёдорович Стравинский'),(66,'Валерий Гергиев'),(67,'Оркестр Мариинского театра')`,
	`INSERT INTO artist_alias (id, artist, name, locale, type) VALUES
		(1,65,'Igor Stravinsky','en',1),
		(2,66,'Valery Gergiev','en',1),
		(3,67,'Kirov Orchestra','en',1),
		(4,67,'Mariinsky Theatre Orchestra','en',1)`,
	`INSERT INTO artist_credit_name (artist_credit, artist) VALUES (66,66)`,
	// The Sacre work, composed by the Cyrillic-named Stravinsky. THREE-level
	// hierarchy (root -> part -> movement): MB movement structures are recursive,
	// and the recordings hang off the deepest level.
	`INSERT INTO work (id, gid, name, type, comment) VALUES
		(350,'w-sacre','Le Sacre du printemps',1,''),
		(351,'w-sacre-p1','Le Sacre du printemps: I. L''Adoration de la terre',1,''),
		(352,'w-sacre-p1-m1','Le Sacre du printemps: I. L''Adoration de la terre: I. Introduction',1,'')`,
	`INSERT INTO work_fts (rowid, title) VALUES
		(350,'Le Sacre du printemps'),
		(351,'Le Sacre du printemps: I. L''Adoration de la terre'),
		(352,'Le Sacre du printemps: I. L''Adoration de la terre: I. Introduction')`,
	`INSERT INTO l_artist_work (id, link, entity0, entity1, link_order) VALUES (40,2,65,350,0)`,
	`INSERT INTO l_work_work (id, link, entity0, entity1, link_order) VALUES
		(13,20,350,351,1),(14,20,351,352,1)`,
	// One Gergiev/Kirov recording of the MOVEMENT (grandchild) work.
	`INSERT INTO recording (id, gid, name, length, comment, artist_credit) VALUES
		(63,'r-sacre-gergiev','Le Sacre du printemps',2040000,'',66)`,
	`INSERT INTO l_recording_work (id, link, entity0, entity1, link_order) VALUES (33,1,63,352,0)`,
	`INSERT INTO l_artist_recording (id, link, entity0, entity1, link_order, entity0_credit) VALUES
		(42,30,66,63,0,''),(43,31,67,63,0,'')`,
	`INSERT INTO release_group (id, gid, name, artist_credit, discogs_master_id) VALUES (73,'rg-sacre-gergiev','Stravinsky: Le Sacre du printemps',66,0)`,
	`INSERT INTO release_group_meta (id, first_release_date_year) VALUES (73,1999)`,
	`INSERT INTO release (id, gid, name, artist_credit, release_group, status, discogs_release_id) VALUES (513,'rel-sacre-gergiev','Stravinsky: Le Sacre du printemps',66,73,1,NULL)`,
	`INSERT INTO medium (id, release, position, format, track_count) VALUES (713,513,1,2,1)`,
	`INSERT INTO track (id, gid, recording, medium, position, number, name, length) VALUES
		(850,'t-sacre',63,713,1,'1','Le Sacre du printemps',2040000)`,
}

// mbEnsembleAliasStmts (Task 1): role-aware artist resolution over merged
// FTS+alias candidates.
//
// Berliner Philharmoniker (68, type 5 Orchestra) carries the artist_alias
// "Berlin Philharmonic" — its German primary name shares no tokens with that
// query at all, so an FTS search for "Berlin Philharmonic" never returns it. A
// decoy "Berlin Philharmonic Wind Quintet" (69, type 2 Group) is the literal FTS
// top hit (the phrase is a substring of its name) and must NOT win: alias-exact
// outranks any FTS rank.
//
// "Leningrad Philharmonic Trio" (70, type 2 Group) and "Leningrad Philharmonic
// Orchestra" (71, type 5 Orchestra) tie on FTS bm25 — same query-term frequency,
// same document length (3 tokens each) — so the lower rowid (70, the trio) wins
// the tie-break (ascending-rowid tie-break verified empirically, see
// mbGenericTitleFloodStmts). Neither carries an alias, so this pair isolates
// role-specific type preference: an orchestra credit must promote 71 past the
// FTS-favored trio; a soloist credit has no reason to and must leave 70 in place.
var mbEnsembleAliasStmts = []string{
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (68,'a-berliner-phil','Berliner Philharmoniker','',5)`,
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (69,'a-berlin-phil-quintet','Berlin Philharmonic Wind Quintet','',2)`,
	`INSERT INTO artist_fts (rowid, name) VALUES (68,'Berliner Philharmoniker'),(69,'Berlin Philharmonic Wind Quintet')`,
	`INSERT INTO artist_alias (id, artist, name, locale, type) VALUES (5,68,'Berlin Philharmonic','en',1)`,
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (70,'a-leningrad-trio','Leningrad Philharmonic Trio','',2)`,
	`INSERT INTO artist (id, gid, name, comment, type) VALUES (71,'a-leningrad-orch','Leningrad Philharmonic Orchestra','',5)`,
	`INSERT INTO artist_fts (rowid, name) VALUES (70,'Leningrad Philharmonic Trio'),(71,'Leningrad Philharmonic Orchestra')`,
}

// mbWorkAliasStmts (Task 2): work_alias candidates in work-group resolution.
//
// English work titles live mostly on movement-level MB works, not the family
// root — work_alias is unioned into resolveWorkGroup's candidate ids so a
// title-FTS miss on the root still recovers the family via a movement alias +
// the existing l_work_work 281 parent walk (step c).
//
// The Sacre root (350, "Le Sacre du printemps") carries no English alias; the
// movement (351) does ("The Rite of Spring: Part I: ..."). That movement also
// gains its own direct composer credit (Stravinsky, artist 65) here: the
// pre-existing Sacre fixture (mbAliasStmts) credits ONLY the root, which is
// realistic for some MB works but means an alias-surfaced movement candidate
// would otherwise fail resolveWorkGroup step (b)'s unmodified composer filter
// (that filter runs on the raw candidate id, before the step (c) root walk).
//
// The Matthäus-Passion, BWV 244 family (root 353 + movement 354, composer Bach
// 64) and its sibling Johannes-Passion, BWV 245 family (root 355 + movement
// 356, also composer Bach) are both minimal: root + one movement + a direct
// composer arc on each. Only the Matthäus movement carries a work_alias row —
// the real-world REAL-mirror shape, punctuation included: "St. Matthew
// Passion, BWV 244: Part I". The query "St Matthew Passion" carries no
// punctuation, so a bare core.NormalizeName prefix comparison would fail on
// the period after "St" alone; workAliasCandidates instead compares via
// foldWorkTitle (catalog/work.go), which strips punctuation after the
// core.NormalizeName fold so the query is still a prefix of the punctuated
// alias. The Johannes sibling carries NO alias at all — it exists so a query
// for the Matthäus family can never accidentally resolve to it (same-composer
// siblings are exactly the case composer arcs alone cannot discriminate).
var mbWorkAliasStmts = []string{
	`CREATE TABLE work_alias (id INTEGER PRIMARY KEY, work INTEGER, name TEXT, locale TEXT, type INTEGER)`,
	`INSERT INTO l_artist_work (id, link, entity0, entity1, link_order) VALUES (41,2,65,351,0)`,
	`INSERT INTO work_alias (id, work, name, locale, type) VALUES
		(1,351,'The Rite of Spring: Part I: Adoration of the Earth','en',1)`,
	`INSERT INTO work (id, gid, name, type, comment) VALUES
		(353,'w-matthaus','Matthäus-Passion, BWV 244',1,''),
		(354,'w-matthaus-p1','Matthäus-Passion, BWV 244: Erster Teil',1,''),
		(355,'w-johannes','Johannes-Passion, BWV 245',1,''),
		(356,'w-johannes-p1','Johannes-Passion, BWV 245: Erster Teil',1,'')`,
	`INSERT INTO work_fts (rowid, title) VALUES
		(353,'Matthäus-Passion, BWV 244'),
		(354,'Matthäus-Passion, BWV 244: Erster Teil'),
		(355,'Johannes-Passion, BWV 245'),
		(356,'Johannes-Passion, BWV 245: Erster Teil')`,
	`INSERT INTO l_artist_work (id, link, entity0, entity1, link_order) VALUES
		(42,2,64,353,0),(43,2,64,354,0),(44,2,64,355,0),(45,2,64,356,0)`,
	`INSERT INTO l_work_work (id, link, entity0, entity1, link_order) VALUES
		(15,20,353,354,1),(16,20,355,356,1)`,
	`INSERT INTO work_alias (id, work, name, locale, type) VALUES
		(2,354,'St. Matthew Passion, BWV 244: Part I','en',1)`,
}

// genericTitleFloodTitle is the exact title shared by work 310 (the real Beethoven
// Symphony No. 5, see mbStmts) and every decoy work below — an exact text match so
// their FTS bm25 scores tie with the real work's, reproducing a generic-title flood.
const genericTitleFloodTitle = "Symphony no. 5 in C minor, op. 67"

// genericTitleFloodComposer is the decoy composer id (mb artist), credited on every
// flood work instead of Beethoven (artist 50).
const genericTitleFloodComposer = 900

// mbGenericTitleFloodStmts seeds a wall of 30 decoy works — same exact title as the
// real Beethoven Symphony No. 5 (work 310), composed by an unrelated decoy composer
// — with ids chosen BELOW 310. FTS5 bm25 ties (identical title text) break by
// ascending rowid (verified empirically), so every decoy outranks work 310 and the
// real work falls outside a naive top-25 candidate window. This reproduces the
// production symptom: a generic title ("Symphony No. 5") drowns among hundreds of
// same-titled works by other composers, and the true composer's work never enters
// the FTS candidate window that resolveWorkGroup filters by composer.
func mbGenericTitleFloodStmts() []string {
	stmts := []string{
		fmt.Sprintf(`INSERT INTO artist (id, gid, name, comment, type) VALUES (%d, 'a-flood-composer', 'Anton Fluter', '', 1)`,
			genericTitleFloodComposer),
	}
	for id := int64(200); id < 230; id++ { // 30 decoys, all id < 310 (the real work's id)
		stmts = append(stmts,
			fmt.Sprintf(`INSERT INTO work (id, gid, name, type, comment) VALUES (%d, 'w-flood-%d', %q, 1, '')`,
				id, id, genericTitleFloodTitle),
			fmt.Sprintf(`INSERT INTO work_fts (rowid, title) VALUES (%d, %q)`, id, genericTitleFloodTitle),
			fmt.Sprintf(`INSERT INTO l_artist_work (id, link, entity0, entity1, link_order) VALUES (%d, 2, %d, %d, 0)`,
				1000+id, genericTitleFloodComposer, id),
		)
	}
	return stmts
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
	`CREATE TABLE track_artist (id INTEGER PRIMARY KEY, track_id INTEGER, artist_id INTEGER, role TEXT, kind TEXT)`,
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
	// --- performance-side bridge tables (Task 3): release_artist, release_identifier, label, label_relationship ---
	`CREATE TABLE release_artist (id INTEGER PRIMARY KEY, release_id INTEGER, seq INTEGER, artist_id INTEGER, role TEXT, kind TEXT)`,
	`CREATE TABLE release_identifier (release_id INTEGER, seq INTEGER, type TEXT, value TEXT, description TEXT)`,
	`CREATE TABLE label (id INTEGER PRIMARY KEY, name TEXT)`,
	`CREATE TABLE label_relationship (parent_label_id INTEGER, sublabel_id INTEGER)`,
	// Discogs artists (bridge targets): 299702 Bernstein, 950 NYPhil.
	`INSERT INTO artist (id, name) VALUES (299702,'Leonard Bernstein'),(950,'New York Philharmonic'),(951,'Wiener Philharmoniker'),(952,'Ludwig van Beethoven'),(953,'Gustav Mahler')`,
	// Labels + a family (Columbia is the parent of CBS).
	`INSERT INTO label (id, name) VALUES (10,'Columbia'),(11,'CBS'),(12,'Deutsche Grammophon')`,
	`INSERT INTO label_relationship (parent_label_id, sublabel_id) VALUES (10,11)`,
	// MASTER A (1963) on CBS (sublabel of Columbia); MASTER B (1985) on Deutsche Grammophon.
	`INSERT INTO master (id, main_release_id, title, year) VALUES (70000,60000,'Beethoven: Symphony No. 5',1963)`,
	`INSERT INTO master (id, main_release_id, title, year) VALUES (70001,60001,'Beethoven: Symphony No. 5',1985)`,
	`INSERT INTO master (id, main_release_id, title, year) VALUES (70002,60002,'Mahler: Symphony No. 5',1961)`,
	`INSERT INTO master_fts (rowid, title, artist_names) VALUES
		(70000,'Beethoven: Symphony No. 5','Leonard Bernstein'),
		(70001,'Beethoven: Symphony No. 5','Leonard Bernstein')`,
	`INSERT INTO release (id, master_id, is_main_release, title, country, released_raw) VALUES
		(60000,70000,1,'Beethoven: Symphony No. 5','US','1963'),
		(60001,70001,1,'Beethoven: Symphony No. 5','DE','1985')`,
	`INSERT INTO release (id, master_id, is_main_release, title, country, released_raw) VALUES (60002,70002,1,'Mahler: Symphony No. 5','US','1961')`,
	`INSERT INTO release_artist (id, release_id, seq, artist_id, role, kind) VALUES
		(1,60000,1,952,'Composed By','artist'),
		(2,60000,2,299702,'Conductor','artist'),
		(3,60000,3,950,'Orchestra','artist'),
		(4,60001,1,952,'Composed By','artist'),
		(5,60001,2,299702,'Conductor, Producer','artist'),
		(6,60002,1,953,'Composed By','artist'),
		(7,60002,2,299702,'Conductor','artist'),
		(8,60002,3,950,'Orchestra','artist')`,
	`INSERT INTO release_label (release_id, seq, label_id, name, catno) VALUES
		(60000,1,11,'CBS','MS 6468'),
		(60001,1,12,'Deutsche Grammophon','DG 415 861-2')`,
	`INSERT INTO release_identifier (release_id, seq, type, value, description) VALUES
		(60001,1,'Barcode','028941586124','')`,
	// Movement tracks for the performer-driven candidates (work-group evidence).
	`INSERT INTO track (id, release_id, parent_track_id, seq, position, title, duration) VALUES
		(120,60000,NULL,1,'A1','Symphony No. 5: I. Allegro con brio','7:22'),
		(121,60000,NULL,2,'A2','Symphony No. 5: II. Andante con moto','10:05'),
		(130,60001,NULL,1,'1','Symphony No. 5: I. Allegro con brio','7:10'),
		(140,60002,NULL,1,'A1','Symphony No. 5: I. Trauermarsch','12:40'),
		(141,60002,NULL,2,'B1','Fidelio Overture','6:30')`,
	// The Mahler decoy's matched group is track-credited to Mahler; the album also
	// carries a release-level Beethoven filler credit (the multi-composer trap).
	`INSERT INTO track_artist (id, track_id, artist_id, role, kind) VALUES
		(1,140,953,'Composed By','extraartist'),
		(2,141,952,'Composed By','extraartist')`,
	`INSERT INTO release_artist (id, release_id, seq, artist_id, role, kind) VALUES
		(9,60002,4,952,'Composed By','artist')`,
}
