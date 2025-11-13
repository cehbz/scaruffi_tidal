"""Tests for canonical performer and label lists"""

import unittest
from domain.canonical import (
    is_canonical_performer, is_canonical_label,
    get_canonical_performer_score, get_canonical_label_score,
    CANONICAL_CONDUCTORS, CANONICAL_PIANISTS
)


class TestCanonicalLists(unittest.TestCase):
    """Test canonical performer and label recognition"""
    
    def test_recognizes_canonical_conductor(self):
        """Should recognize canonical conductors"""
        self.assertTrue(is_canonical_performer("Karajan"))
        self.assertTrue(is_canonical_performer("Bernstein"))
        self.assertTrue(is_canonical_performer("Gardiner"))
    
    def test_recognizes_full_conductor_name(self):
        """Should recognize conductors with full names"""
        self.assertTrue(is_canonical_performer("Herbert von Karajan"))
        self.assertTrue(is_canonical_performer("Leonard Bernstein"))
        self.assertTrue(is_canonical_performer("John Eliot Gardiner"))
    
    def test_recognizes_canonical_pianist(self):
        """Should recognize canonical pianists"""
        self.assertTrue(is_canonical_performer("Gould"))
        self.assertTrue(is_canonical_performer("Pollini"))
        self.assertTrue(is_canonical_performer("Richter"))
    
    def test_recognizes_full_pianist_name(self):
        """Should recognize pianists with full names"""
        self.assertTrue(is_canonical_performer("Glenn Gould"))
        self.assertTrue(is_canonical_performer("Sviatoslav Richter"))
        self.assertTrue(is_canonical_performer("Maurizio Pollini"))
    
    def test_recognizes_canonical_ensemble(self):
        """Should recognize canonical ensembles"""
        self.assertTrue(is_canonical_performer("Alban Berg Quartet"))
        self.assertTrue(is_canonical_performer("Il Giardino Armonico"))
        self.assertTrue(is_canonical_performer("Tallis Scholars"))
    
    def test_recognizes_canonical_orchestra(self):
        """Should recognize canonical orchestras"""
        self.assertTrue(is_canonical_performer("Berliner Philharmoniker"))
        self.assertTrue(is_canonical_performer("Berlin Philharmonic"))
        self.assertTrue(is_canonical_performer("Vienna Philharmonic"))
        self.assertTrue(is_canonical_performer("Wiener Philharmoniker"))
    
    def test_rejects_non_canonical_performer(self):
        """Should reject unknown performers"""
        self.assertFalse(is_canonical_performer("Unknown Orchestra"))
        self.assertFalse(is_canonical_performer("Random Conductor"))
    
    def test_case_insensitive_matching(self):
        """Should match case-insensitively"""
        self.assertTrue(is_canonical_performer("karajan"))
        self.assertTrue(is_canonical_performer("BERNSTEIN"))
        self.assertTrue(is_canonical_performer("gOuLd"))
    
    def test_recognizes_canonical_label(self):
        """Should recognize canonical labels"""
        self.assertTrue(is_canonical_label("Deutsche Grammophon"))
        self.assertTrue(is_canonical_label("DG"))
        self.assertTrue(is_canonical_label("Decca"))
        self.assertTrue(is_canonical_label("ECM"))
        self.assertTrue(is_canonical_label("Hyperion"))
    
    def test_recognizes_label_with_suffix(self):
        """Should recognize labels with additional text"""
        self.assertTrue(is_canonical_label("Sony Classical"))
        self.assertTrue(is_canonical_label("Warner Classics"))
    
    def test_rejects_non_canonical_label(self):
        """Should reject unknown labels"""
        self.assertFalse(is_canonical_label("Unknown Records"))
        self.assertFalse(is_canonical_label("Random Label"))
    
    def test_performer_score(self):
        """Should return correct score for performers"""
        self.assertEqual(get_canonical_performer_score("Karajan"), 1.0)
        self.assertEqual(get_canonical_performer_score("Unknown"), 0.0)
    
    def test_label_score(self):
        """Should return correct score for labels"""
        self.assertEqual(get_canonical_label_score("DG"), 1.0)
        self.assertEqual(get_canonical_label_score("Unknown"), 0.0)
    
    def test_empty_string_handling(self):
        """Should handle empty strings gracefully"""
        self.assertFalse(is_canonical_performer(""))
        self.assertFalse(is_canonical_label(""))
        self.assertEqual(get_canonical_performer_score(""), 0.0)
        self.assertEqual(get_canonical_label_score(""), 0.0)
    
    def test_canonical_sets_not_empty(self):
        """Should have non-empty canonical lists"""
        self.assertGreater(len(CANONICAL_CONDUCTORS), 20)
        self.assertGreater(len(CANONICAL_PIANISTS), 10)


if __name__ == '__main__':
    unittest.main()
