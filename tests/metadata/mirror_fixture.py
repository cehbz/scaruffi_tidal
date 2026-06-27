"""Build tiny MusicBrainz + Discogs SQLite fixtures for offline mirror tests."""
import sqlite3
from pathlib import Path

# (artist_id, gid, name, comment)
_ARTIST = (9133, "9fadfba9-ecae-4383-a4d8-47b043cea19a", "Traffic", "English rock band")
# release_group 60527, artist_credit 9133 (single-artist: ac.id == artist.id here)
_RG = (60527, "3770d5ce-e0e1-3389-9acf-cd38f0722baf", "John Barleycorn Must Die", 9133, 69017)
_REL = (5613961, "33230f12-77ab-4076-8407-8b8f04b96336", "John Barleycorn Must Die", 9133, 60527, "Official", 14092282)
# (track_pos, rec_id, rec_gid, title, length_ms, isrc)
_TRACKS = [
    (1, 4432211, "53bb54ac-1020-4cd4-83ce-362a58e1ec17", "Glad", 419093, "GBUM71030667"),
    (2, 4432212, "a661dc41-618c-4ecb-9555-712fc29883a1", "Freedom Rider", 329266, "GBUM71030668"),
    (3, 4432213, "3ff13db1-6317-43ac-8fc9-5feeaeee00f8", "Empty Pages", 276640, "GBUM71030669"),
    (4, 4432214, "17e9d03f-584f-44c1-9221-d7455fbb3697", "Stranger to Himself", 235666, "GBUM71030670"),
    (5, 4432215, "dfe6a94e-d5ff-4fac-87fd-262d855a6a93", "John Barleycorn", 386426, "GBUM71030671"),
    (6, 4432216, "b4fca1f7-87b0-451e-8e08-1a66151970d3", "Every Mother's Son", 425600, "GBUM71030672"),
]


def build_mirror_fixture(tmp_path) -> tuple[str, str]:
    mb_path = str(Path(tmp_path) / "musicbrainz.db")
    dc_path = str(Path(tmp_path) / "discogs.db")
    _build_mb(mb_path)
    _build_discogs(dc_path)
    return mb_path, dc_path


def _build_mb(path: str) -> None:
    con = sqlite3.connect(path)
    con.executescript(
        """
        CREATE TABLE artist(id INTEGER PRIMARY KEY, gid TEXT, name TEXT, comment TEXT,
                            discogs_artist_id INTEGER);
        CREATE TABLE artist_credit_name(artist_credit INTEGER, position INTEGER,
                            artist INTEGER, name TEXT, join_phrase TEXT);
        CREATE TABLE recording(id INTEGER PRIMARY KEY, gid TEXT, name TEXT,
                            artist_credit INTEGER, length INTEGER, comment TEXT);
        CREATE TABLE release_group_meta(id INTEGER PRIMARY KEY,
                            first_release_date_year INTEGER);
        CREATE TABLE release_group_secondary_type(id INTEGER PRIMARY KEY, name TEXT);
        CREATE TABLE release_group_secondary_type_join(release_group INTEGER,
                            secondary_type INTEGER);
        CREATE TABLE isrc(id INTEGER PRIMARY KEY, recording INTEGER, isrc TEXT);
        CREATE TABLE release_group(id INTEGER PRIMARY KEY, gid TEXT, name TEXT,
                            artist_credit INTEGER, type INTEGER, discogs_master_id INTEGER);
        CREATE TABLE release(id INTEGER PRIMARY KEY, gid TEXT, name TEXT, artist_credit INTEGER,
                            release_group INTEGER, status TEXT, discogs_release_id INTEGER);
        CREATE TABLE medium(id INTEGER PRIMARY KEY, release INTEGER, position INTEGER,
                            format INTEGER, track_count INTEGER);
        CREATE TABLE track(id INTEGER PRIMARY KEY, gid TEXT, recording INTEGER, medium INTEGER,
                            position INTEGER, number TEXT, name TEXT, artist_credit INTEGER,
                            length INTEGER);
        CREATE TABLE canonical_musicbrainz_data(id INTEGER PRIMARY KEY, artist_credit_id INTEGER,
                            artist_mbids TEXT, artist_credit_name TEXT, release_mbid TEXT,
                            release_name TEXT, recording_mbid TEXT, recording_name TEXT,
                            combined_lookup TEXT, score INTEGER);
        CREATE VIRTUAL TABLE artist_fts USING fts5(name, content='');
        CREATE VIRTUAL TABLE recording_fts USING fts5(title, artist_names, content='');
        CREATE VIRTUAL TABLE release_group_fts USING fts5(title, artist_names, content='');
        """
    )
    con.execute("INSERT INTO artist VALUES(?,?,?,?,NULL)", _ARTIST)
    con.execute("INSERT INTO artist_credit_name VALUES(?,?,?,?,'')", (9133, 1, 9133, "Traffic"))
    con.execute("INSERT INTO release_group VALUES(?,?,?,?,1,?)", _RG)
    con.execute("INSERT INTO release VALUES(?,?,?,?,?,?,?)", _REL)
    con.execute("INSERT INTO medium VALUES(1, 5613961, 1, 1, 6)")
    con.execute("INSERT INTO artist_fts(rowid, name) VALUES(9133, 'Traffic')")
    con.execute("INSERT INTO release_group_fts(rowid, title, artist_names) VALUES(60527, ?, 'Traffic')",
                ("John Barleycorn Must Die",))
    for pos, rid, rgid, title, length, isrc in _TRACKS:
        con.execute("INSERT INTO recording VALUES(?,?,?,9133,?,?)", (rid, rgid, title, length, ""))
        con.execute("INSERT INTO isrc(recording, isrc) VALUES(?,?)", (rid, isrc))
        con.execute("INSERT INTO track VALUES(?,?,?,1,?,?,?,9133,?)",
                    (rid, rgid + "-t", rid, pos, str(pos), title, length))
        con.execute("INSERT INTO recording_fts(rowid, title, artist_names) VALUES(?,?,'Traffic')",
                    (rid, title))
        con.execute("INSERT INTO canonical_musicbrainz_data(artist_credit_id, release_mbid, "
                    "recording_mbid, score) VALUES(9133, ?, ?, 100)",
                    (_REL[1], rgid))

    # Secondary type reference rows
    con.execute("INSERT INTO release_group_secondary_type VALUES(1, 'Compilation')")
    con.execute("INSERT INTO release_group_secondary_type VALUES(6, 'Live')")

    # release_group_meta for JBMD and compilation RG
    con.execute("INSERT INTO release_group_meta VALUES(60527, 1970)")
    con.execute("INSERT INTO release_group_meta VALUES(55885, 1976)")

    # Standalone LIVE recording (no track/canonical rows — only for recordings_for)
    con.execute(
        "INSERT INTO recording VALUES(?,?,?,?,?,?)",
        (9990001, "aaaa1111-0000-0000-0000-000000000001", "Pearly Queen", 9133, 300000,
         "live, 1994: Woodstock"),
    )
    con.execute(
        "INSERT INTO recording_fts(rowid, title, artist_names) VALUES(?,?,'Traffic')",
        (9990001, "Pearly Queen"),
    )

    # Compilation release-group with no releases/tracks/canonical → empty tracklist
    con.execute(
        "INSERT INTO release_group VALUES(?,?,?,?,1,NULL)",
        (55885, "bbbb2222-0000-0000-0000-000000000002", "Best of Traffic", 9133),
    )
    con.execute(
        "INSERT INTO release_group_fts(rowid, title, artist_names) VALUES(?,?,'Traffic')",
        (55885, "Best of Traffic"),
    )
    con.execute("INSERT INTO release_group_secondary_type_join VALUES(55885, 1)")

    con.commit()
    con.close()


def _build_discogs(path: str) -> None:
    con = sqlite3.connect(path)
    con.executescript("""
        CREATE TABLE artist(id INTEGER PRIMARY KEY, name TEXT);
        CREATE TABLE master(id INTEGER PRIMARY KEY, main_release_id INTEGER,
                            title TEXT, year INTEGER, data_quality TEXT);
        CREATE TABLE master_artist(master_id INTEGER, seq INTEGER, artist_id INTEGER,
                                   anv TEXT, join_str TEXT, role TEXT,
                                   PRIMARY KEY(master_id, seq));
        CREATE TABLE master_genre(master_id INTEGER, seq INTEGER, genre TEXT NOT NULL,
                                  PRIMARY KEY(master_id, seq));
        CREATE TABLE master_style(master_id INTEGER, seq INTEGER, style TEXT NOT NULL,
                                  PRIMARY KEY(master_id, seq));
        CREATE TABLE release(id INTEGER PRIMARY KEY, master_id INTEGER, is_main_release INTEGER);
        CREATE TABLE track(id INTEGER PRIMARY KEY, release_id INTEGER, seq INTEGER,
                           position TEXT, title TEXT, duration TEXT);
        CREATE VIRTUAL TABLE master_fts USING fts5(title, artist_names, content='');
    """)

    # Artist: Traffic
    con.execute("INSERT INTO artist VALUES(271556, 'Traffic')")

    # Master: John Barleycorn Must Die (69017)
    con.execute("INSERT INTO master VALUES(69017, 583800, 'John Barleycorn Must Die', 1970, 'Correct')")
    con.execute("INSERT INTO master_artist VALUES(69017, 1, 271556, '', '', '')")

    # Genres and styles
    con.execute("INSERT INTO master_genre VALUES(69017, 1, 'Rock')")
    con.execute("INSERT INTO master_style VALUES(69017, 1, 'Folk Rock')")
    con.execute("INSERT INTO master_style VALUES(69017, 2, 'Blues Rock')")
    con.execute("INSERT INTO master_style VALUES(69017, 3, 'Prog Rock')")

    # Main release
    con.execute("INSERT INTO release VALUES(583800, 69017, 1)")

    # Tracks for release 583800
    _DC_TRACKS = [
        (1, 583800, 1, "A1", "Glad",                 "6:32"),
        (2, 583800, 2, "A2", "Freedom Rider",         "6:20"),
        (3, 583800, 3, "A3", "Empty Pages",           "4:47"),
        (4, 583800, 4, "B1", "Stranger To Himself",   "4:02"),
        (5, 583800, 5, "B2", "John Barleycorn",       "6:20"),
        (6, 583800, 6, "B3", "Every Mother's Son",    "7:05"),
    ]
    for row in _DC_TRACKS:
        con.execute("INSERT INTO track VALUES(?,?,?,?,?,?)", row)

    # FTS row (contentless — insert via special syntax)
    con.execute("INSERT INTO master_fts(rowid, title, artist_names) VALUES(69017, ?, ?)",
                ("John Barleycorn Must Die", "Traffic"))

    con.commit()
    con.close()
