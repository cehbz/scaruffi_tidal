"""Tests for Discogs client"""

import unittest
from unittest.mock import Mock, MagicMock, patch
from infrastructure.discogs_client import DiscogsClient
from domain.recording import Recording
from domain.discogs import DiscogsRelease, DiscogsSearchResult


class TestDiscogsClient(unittest.TestCase):
    """Test Discogs API client"""
    
    def setUp(self):
        """Create mock Discogs client"""
        self.mock_api = Mock()
        self.client = DiscogsClient(token='test_token')
        self.client._client = self.mock_api
    
    def test_search_recording_basic(self):
        """Should search for a recording and return results"""
        recording = Recording(
            composer="Bach",
            work="Brandenburg Concertos",
            performer="Il Giardino Armonico",
            year=1997
        )
        
        # Mock Discogs API response
        mock_artist = Mock()
        mock_artist.name = "Il Giardino Armonico"
        
        mock_label = Mock()
        mock_label.name = "Teldec"
        
        mock_result = Mock()
        mock_result.id = 12345
        mock_result.title = "Brandenburg Concertos"
        mock_result.artists = [mock_artist]
        mock_result.year = 1997
        mock_result.labels = [mock_label]
        mock_result.formats = [{"name": "CD"}]
        mock_result.master_id = 54321
        mock_result.type = "release"
        mock_result.community = Mock(
            rating=Mock(average=4.5),
            have=100,
            want=50
        )
        
        self.mock_api.search.return_value.page.return_value = [mock_result]
        
        result = self.client.search_recording(recording)
        
        self.assertIsNotNone(result)
        self.assertTrue(result.found_exact_match)
        self.assertEqual(result.discogs_release.id, 12345)
        self.assertEqual(result.discogs_release.year, 1997)
    
    def test_search_recording_no_results(self):
        """Should handle no results found"""
        recording = Recording(
            composer="Unknown",
            work="Nonexistent Work"
        )
        
        self.mock_api.search.return_value.page.return_value = []
        
        result = self.client.search_recording(recording)
        
        self.assertIsNotNone(result)
        self.assertFalse(result.found_exact_match)
        self.assertIsNone(result.discogs_release)
    
    def test_search_finds_master(self):
        """Should handle master releases"""
        recording = Recording(
            composer="Bach",
            work="Brandenburg Concertos",
            performer="Il Giardino Armonico"
        )
        
        # Mock master release
        mock_artist = Mock()
        mock_artist.name = "Il Giardino Armonico"
        
        mock_master = Mock()
        mock_master.id = 99999
        mock_master.title = "Brandenburg Concertos"
        mock_master.artists = [mock_artist]
        mock_master.year = 1997
        mock_master.data = {}
        mock_master.type = "master"
        mock_master.labels = []
        mock_master.formats = []
        mock_master.community = None
        
        self.mock_api.search.return_value.page.return_value = [mock_master]
        
        result = self.client.search_recording(recording)
        
        self.assertTrue(result.found_exact_match)
        self.assertTrue(result.discogs_release.is_master)
    
    def test_respects_rate_limit(self):
        """Should use rate limiter for requests"""
        recording = Recording(composer="Test", work="Test")
        
        self.mock_api.search.return_value.page.return_value = []
        
        # Make multiple requests
        for _ in range(3):
            self.client.search_recording(recording)
        
        # Should have called search 3 times
        self.assertEqual(self.mock_api.search.call_count, 3)
    
    def test_filters_by_metadata(self):
        """Should filter results by performer/label/year"""
        recording = Recording(
            composer="Bach",
            work="Goldberg Variations",
            performer="Glenn Gould",
            label="Sony",
            year=1955
        )
        
        # Mock two results - one matching, one not
        mock_artist1 = Mock()
        mock_artist1.name = "Glenn Gould"
        mock_label1 = Mock()
        mock_label1.name = "Sony Classical"
        
        mock_match = Mock()
        mock_match.id = 11111
        mock_match.title = "Goldberg Variations"
        mock_match.artists = [mock_artist1]
        mock_match.year = 1955
        mock_match.labels = [mock_label1]
        mock_match.formats = []
        mock_match.type = "release"
        mock_match.master_id = None
        mock_match.community = Mock(rating=Mock(average=5.0), have=500, want=100)
        
        mock_artist2 = Mock()
        mock_artist2.name = "András Schiff"
        mock_label2 = Mock()
        mock_label2.name = "ECM"
        
        mock_no_match = Mock()
        mock_no_match.id = 22222
        mock_no_match.title = "Goldberg Variations"
        mock_no_match.artists = [mock_artist2]
        mock_no_match.year = 2001
        mock_no_match.labels = [mock_label2]
        mock_no_match.formats = []
        mock_no_match.type = "release"
        mock_no_match.master_id = None
        mock_no_match.community = Mock(rating=Mock(average=4.8), have=300, want=80)
        
        self.mock_api.search.return_value.page.return_value = [mock_no_match, mock_match]
        
        result = self.client.search_recording(recording)
        
        # Should find the matching one
        self.assertTrue(result.found_exact_match)
        self.assertEqual(result.discogs_release.id, 11111)
        self.assertIn("Glenn Gould", result.discogs_release.artists)


if __name__ == '__main__':
    unittest.main()
