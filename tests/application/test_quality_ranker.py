"""Tests for quality-based ranking"""

import unittest
from domain.tidal import TidalAlbum
from domain.discogs import DiscogsRelease
from domain.recording import Recording
from application.quality_ranker import QualityRanker


class TestQualityRanker(unittest.TestCase):
    """Test quality-based ranking algorithm"""
    
    def setUp(self):
        self.ranker = QualityRanker()
        self.recording = Recording(
            composer="Bach",
            work="Brandenburg Concertos",
            performer="Il Giardino Armonico",
            year=1997
        )
    
    def test_exact_match_scores_perfect(self):
        """Exact match to Discogs should score 1.0"""
        discogs = DiscogsRelease(
            id=12345,
            title="Brandenburg Concertos",
            artists=("Il Giardino Armonico",),
            year=1997,
            labels=("Teldec",)
        )
        
        tidal = TidalAlbum(
            id=99999,
            title="Bach: Brandenburg Concertos",
            artists=("Il Giardino Armonico",),
            release_date="1997-06-15",
            popularity=50
        )
        
        score = self.ranker.score_album(tidal, self.recording, discogs, max_popularity=100)
        
        self.assertEqual(score, 1.0)
    
    def test_canonical_performer_high_score(self):
        """Canonical performer should score high"""
        tidal = TidalAlbum(
            id=1,
            title="Brandenburg Concertos",
            artists=("Il Giardino Armonico",),  # Canonical ensemble
            popularity=50
        )
        
        score = self.ranker.score_album(tidal, self.recording, max_popularity=100)
        
        # Should get high score from canonical performer (0.5 weight)
        self.assertGreater(score, 0.5)
    
    def test_non_canonical_performer_lower_score(self):
        """Non-canonical performer should score lower"""
        tidal = TidalAlbum(
            id=1,
            title="Brandenburg Concertos",
            artists=("Unknown Orchestra",),
            popularity=50
        )
        
        score = self.ranker.score_album(tidal, self.recording, max_popularity=100)
        
        # Should only get popularity score (0.15 weight * 0.5 normalized)
        self.assertLess(score, 0.2)
    
    def test_popularity_tiebreaker(self):
        """Popularity should break ties between similar albums"""
        album1 = TidalAlbum(
            id=1,
            title="Brandenburg Concertos",
            artists=("Unknown Orchestra",),
            popularity=100
        )
        
        album2 = TidalAlbum(
            id=2,
            title="Brandenburg Concertos",
            artists=("Unknown Orchestra",),
            popularity=50
        )
        
        score1 = self.ranker.score_album(album1, self.recording, max_popularity=100)
        score2 = self.ranker.score_album(album2, self.recording, max_popularity=100)
        
        self.assertGreater(score1, score2)
    
    def test_rank_albums_sorts_by_score(self):
        """Should rank albums by descending score"""
        albums = [
            TidalAlbum(id=1, title="Test", artists=("Unknown",), popularity=10),
            TidalAlbum(id=2, title="Test", artists=("Karajan",), popularity=50),  # Canonical
            TidalAlbum(id=3, title="Test", artists=("Unknown",), popularity=100),
        ]
        
        ranked = self.ranker.rank_albums(albums, self.recording)
        
        # Canonical performer should be first
        self.assertEqual(ranked[0][0].id, 2)
        # Highest score
        self.assertGreater(ranked[0][1], ranked[1][1])
        self.assertGreater(ranked[1][1], ranked[2][1])
    
    def test_find_best_match_returns_highest(self):
        """Should return album with highest score"""
        albums = [
            TidalAlbum(id=1, title="Test", artists=("Unknown",), popularity=50),
            TidalAlbum(id=2, title="Test", artists=("Gardiner",), popularity=50),  # Canonical
        ]
        
        result = self.ranker.find_best_match(albums, self.recording)
        
        self.assertIsNotNone(result)
        best_album, score = result
        self.assertEqual(best_album.id, 2)
        self.assertGreater(score, 0.5)
    
    def test_find_best_match_respects_min_score(self):
        """Should return None if no album meets minimum score"""
        albums = [
            TidalAlbum(id=1, title="Test", artists=("Unknown",), popularity=10),
        ]
        
        result = self.ranker.find_best_match(albums, self.recording, min_score=0.9)
        
        self.assertIsNone(result)
    
    def test_find_best_match_with_exact_match(self):
        """Exact match should always be returned"""
        discogs = DiscogsRelease(
            id=12345,
            title="Brandenburg Concertos",
            artists=("Il Giardino Armonico",),
            year=1997
        )
        
        albums = [
            TidalAlbum(id=1, title="Test", artists=("Unknown",), popularity=100),
            TidalAlbum(
                id=2,
                title="Brandenburg Concertos",
                artists=("Il Giardino Armonico",),
                release_date="1997-06-15",
                popularity=50
            ),
        ]
        
        result = self.ranker.find_best_match(albums, self.recording, discogs)
        
        self.assertIsNotNone(result)
        best_album, score = result
        self.assertEqual(best_album.id, 2)
        self.assertEqual(score, 1.0)
    
    def test_empty_album_list(self):
        """Should handle empty album list"""
        ranked = self.ranker.rank_albums([], self.recording)
        
        self.assertEqual(len(ranked), 0)
        
        result = self.ranker.find_best_match([], self.recording)
        
        self.assertIsNone(result)
    
    def test_multiple_canonical_performers(self):
        """Should use highest score when multiple artists"""
        album = TidalAlbum(
            id=1,
            title="Test",
            artists=("Unknown Orchestra", "Karajan", "Another Unknown"),  # One canonical
            popularity=50
        )
        
        score = self.ranker.score_album(album, self.recording, max_popularity=100)
        
        # Should get full canonical performer score
        self.assertGreater(score, 0.5)


if __name__ == '__main__':
    unittest.main()
