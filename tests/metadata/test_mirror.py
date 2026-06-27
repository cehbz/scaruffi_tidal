import pytest

from tidalist.metadata.mirror import MirrorDB
from tests.metadata.mirror_fixture import build_mirror_fixture


def test_mirrordb_connects_and_attaches_discogs(tmp_path):
    mb, dc = build_mirror_fixture(tmp_path)
    con = MirrorDB(mb, dc).connect()
    dbs = {row[1] for row in con.execute("PRAGMA database_list")}
    assert "main" in dbs and "dc" in dbs
    name = con.execute("SELECT name FROM artist WHERE id=9133").fetchone()[0]
    assert name == "Traffic"
    con.close()


def test_mirrordb_missing_db_fails_loudly(tmp_path):
    _, dc = build_mirror_fixture(tmp_path)
    with pytest.raises(FileNotFoundError) as exc:
        MirrorDB(str(tmp_path / "nope.db"), dc).connect()
    assert "nope.db" in str(exc.value)
