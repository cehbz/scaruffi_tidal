import pytest

from tidalist.core.identifiers import ISRC, MBID, ExternalIds
from tidalist.core.recording import Credit
from tidalist.core.album import Album, TrackRef, ReleaseTrait


def test_album_carries_identity_fields():
    a = Album(artist="Traffic", title="John Barleycorn Must Die",
              ids=ExternalIds(mbid=MBID("rg-1")), first_released=1970)
    assert a.artist == "Traffic" and a.title == "John Barleycorn Must Die"
    assert a.ids.mbid == "rg-1" and a.first_released == 1970


def test_album_optional_fields_default_none():
    a = Album(artist="Blind Faith", title="Blind Faith")
    assert a.ids == ExternalIds() and a.first_released is None


# --- Phase 4 Task 1 (updated): release traits ---

def test_album_carries_traits():
    a = Album(artist="Traffic", title="Live Traffic",
              traits=frozenset({ReleaseTrait.LIVE}))
    assert ReleaseTrait.LIVE in a.traits


def test_album_traits_defaults_empty_frozenset():
    a = Album(artist="Traffic", title="John Barleycorn Must Die")
    assert a.traits == frozenset()


def test_album_can_carry_multiple_traits():
    a = Album(artist="Various", title="Live Comp",
              traits=frozenset({ReleaseTrait.LIVE, ReleaseTrait.COMPILATION}))
    assert ReleaseTrait.LIVE in a.traits and ReleaseTrait.COMPILATION in a.traits


# --- Slice 3 Task 1: album styles ---

def test_album_carries_styles():
    a = Album(artist="Traffic", title="John Barleycorn Must Die",
              styles=frozenset({"Folk Rock"}))
    assert a.styles == frozenset({"Folk Rock"})


def test_album_styles_defaults_empty_frozenset():
    a = Album(artist="Traffic", title="John Barleycorn Must Die")
    assert a.styles == frozenset()


# --- Phase 2 (edition-distance): TrackRef and Album.tracklist ---

def test_trackref_constructs_and_compares_by_value():
    t1 = TrackRef(position=1, title="Glad", isrc=ISRC("GBABC1234567"),
                  mbid=MBID("rec-1"), duration_s=386)
    t2 = TrackRef(position=1, title="Glad", isrc=ISRC("GBABC1234567"),
                  mbid=MBID("rec-1"), duration_s=386)
    assert t1 == t2


def test_trackref_optional_fields_default_none():
    t = TrackRef(position=2, title="Freedom Rider")
    assert t.isrc is None
    assert t.mbid is None
    assert t.duration_s is None


def test_trackref_is_frozen():
    t = TrackRef(position=1, title="Glad")
    with pytest.raises((AttributeError, TypeError)):
        t.title = "Nope"  # type: ignore[misc]


def test_album_with_tracklist_holds_it():
    tracks = (
        TrackRef(position=1, title="Glad", isrc=ISRC("GBABC1234567"), duration_s=386),
        TrackRef(position=2, title="Freedom Rider"),
    )
    a = Album(artist="Traffic", title="John Barleycorn Must Die", tracklist=tracks)
    assert a.tracklist == tracks


def test_album_tracklist_defaults_empty_tuple():
    a = Album(artist="Traffic", title="John Barleycorn Must Die")
    assert a.tracklist == ()


# --- Task 2 (credit-anchored render): Album.credits reuses recording.Credit ---

def test_album_carries_credits():
    a = Album(artist="Wiener Philharmoniker", title="Beethoven: Symphony No. 5",
              credits=(Credit("Carlos Kleiber", "conductor"),
                       Credit("Wiener Philharmoniker", "orchestra")))
    assert a.credits == (Credit("Carlos Kleiber", "conductor"),
                         Credit("Wiener Philharmoniker", "orchestra"))


def test_album_credits_defaults_empty_tuple():
    a = Album(artist="Traffic", title="John Barleycorn Must Die")
    assert a.credits == ()
