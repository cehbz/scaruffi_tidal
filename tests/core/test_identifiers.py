from tidalist.core.identifiers import (
    ExternalIds, Source, MBID, DiscogsMasterId, DiscogsReleaseId,
)


def test_external_ids_defaults_are_empty():
    ids = ExternalIds()
    assert ids.mbid is None
    assert ids.discogs_master_id is None
    assert ids.discogs_release_id is None
    assert ids.sources == frozenset()


def test_external_ids_carries_values():
    ids = ExternalIds(
        mbid=MBID("rg-1"),
        discogs_master_id=DiscogsMasterId(42),
        discogs_release_id=DiscogsReleaseId(99),
        sources=frozenset({Source.MUSICBRAINZ, Source.DISCOGS}),
    )
    assert ids.mbid == "rg-1"
    assert ids.discogs_master_id == 42
    assert ids.discogs_release_id == 99
    assert Source.MUSICBRAINZ in ids.sources and Source.DISCOGS in ids.sources


def test_external_ids_value_equality():
    assert ExternalIds(mbid=MBID("x")) == ExternalIds(mbid=MBID("x"))
    assert ExternalIds(mbid=MBID("x")) != ExternalIds(mbid=MBID("y"))


def test_source_string_values():
    assert Source.MUSICBRAINZ == "musicbrainz"
    assert Source.DISCOGS == "discogs"
