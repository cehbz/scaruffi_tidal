"""Tests for Scaruffi HTML parser"""

import unittest
from pathlib import Path

from infrastructure.scaruffi_parser import ScaruffiParser
from domain.scaruffi_entry import ScaruffiEntry


class TestScaruffiParser(unittest.TestCase):
    """Test Scaruffi HTML parser"""
    
    def setUp(self):
        self.parser = ScaruffiParser()
    
    def test_parse_basic_entry(self):
        """Should parse a basic entry with performer and year"""
        html = """
        <table>
        <tr><td>
        <br>Bach: Brandenburg Concertos 
        <br>Recommended recording: Il Giardino Armonico (1997)
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 1)
        entry = entries[0]
        self.assertEqual(entry.composer, "Bach")
        self.assertEqual(entry.work, "Brandenburg Concertos")
        self.assertEqual(entry.primary_recording.composer, "Bach")
        self.assertEqual(entry.primary_recording.work, "Brandenburg Concertos")
        self.assertEqual(entry.primary_recording.performer, "Il Giardino Armonico")
        self.assertEqual(entry.primary_recording.year, 1997)
        self.assertIsNone(entry.primary_recording.label)
        self.assertEqual(len(entry.alternate_recordings), 0)
    
    def test_parse_entry_with_year_range(self):
        """Should handle year ranges by using first year"""
        html = """
        <table>
        <tr><td>
        <br>Haydn: String Quartets opp. 76; 77; 103
        <br>Recommended recording: Amadeus String Quartet (1963-73)
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 1)
        entry = entries[0]
        self.assertEqual(entry.primary_recording.year, 1963)
    
    def test_parse_entry_with_label(self):
        """Should extract label from parentheses"""
        html = """
        <table>
        <tr><td>
        <br>Bach: Organ Works
        <br>Recommended recording: Masaaki Suzuki (BIS)
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 1)
        entry = entries[0]
        self.assertEqual(entry.primary_recording.performer, "Masaaki Suzuki")
        self.assertEqual(entry.primary_recording.label, "BIS")
        self.assertIsNone(entry.primary_recording.year)
    
    def test_parse_entry_with_conductor_and_orchestra(self):
        """Should handle conductor & orchestra format"""
        html = """
        <table>
        <tr><td>
        <br>Beethoven: The Nine Symphonies
        <br>Recommended recording: Karajan & Berliner Philharmoniker  (1975-77)
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 1)
        entry = entries[0]
        self.assertEqual(entry.primary_recording.performer, "Karajan & Berliner Philharmoniker")
        self.assertEqual(entry.primary_recording.year, 1975)
    
    def test_parse_entry_with_alternates(self):
        """Should parse alternate recordings from 'also' clause"""
        html = """
        <table>
        <tr><td>
        <br>Bach: Brandenburg Concertos 
        <br>Recommended recording: Il Giardino Armonico (1997) (also Trevor Pinnock and European Brandenburg Ensemble)
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 1)
        entry = entries[0]
        self.assertEqual(entry.primary_recording.performer, "Il Giardino Armonico")
        self.assertEqual(len(entry.alternate_recordings), 1)
        self.assertEqual(entry.alternate_recordings[0].performer, "Trevor Pinnock and European Brandenburg Ensemble")
        self.assertEqual(entry.alternate_recordings[0].composer, "Bach")
        self.assertEqual(entry.alternate_recordings[0].work, "Brandenburg Concertos")
    
    def test_parse_entry_with_multiple_alternates(self):
        """Should parse multiple alternates separated by semicolons"""
        html = """
        <table>
        <tr><td>
        <br>Bach: Goldberg Variations 
        <br>Recommended recording: Glenn Gould  (1955) (also Andras Schiff on ECM, Murray Perahia on Sony)
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 1)
        entry = entries[0]
        self.assertEqual(len(entry.alternate_recordings), 2)
        self.assertEqual(entry.alternate_recordings[0].performer, "Andras Schiff")
        self.assertEqual(entry.alternate_recordings[0].label, "ECM")
        self.assertEqual(entry.alternate_recordings[1].performer, "Murray Perahia")
        self.assertEqual(entry.alternate_recordings[1].label, "Sony")
    
    def test_parse_entry_performer_only(self):
        """Should handle entries with just performer name"""
        html = """
        <table>
        <tr><td>
        <br>Beethoven: Sonatas in general (eg Hammerklavier)
        <br>Recommended recording: Sviatoslav Richter
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 1)
        entry = entries[0]
        self.assertEqual(entry.primary_recording.performer, "Sviatoslav Richter")
        self.assertIsNone(entry.primary_recording.year)
        self.assertIsNone(entry.primary_recording.label)
    
    def test_parse_entry_multiple_performers_or(self):
        """Should handle 'or' separated performers"""
        html = """
        <table>
        <tr><td>
        <br>Schubert: Piano Sonata D959/D960
        <br>Recommended recording: Pollini or Krystian Zimerman
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 1)
        entry = entries[0]
        # Should treat 'or' as alternates
        self.assertEqual(entry.primary_recording.performer, "Pollini")
        self.assertEqual(len(entry.alternate_recordings), 1)
        self.assertEqual(entry.alternate_recordings[0].performer, "Krystian Zimerman")
    
    def test_parse_multiple_entries(self):
        """Should parse multiple entries from one HTML document"""
        html = """
        <table>
        <tr><td>
        <br>Bach: Brandenburg Concertos 
        <br>Recommended recording: Il Giardino Armonico (1997)
        <br>
        <br>Mozart: Requiem, K.626
        <br>Recommended recording: Gardiner & Monteverdi Choir & English Baroque Soloists (1986)
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 2)
        self.assertEqual(entries[0].composer, "Bach")
        self.assertEqual(entries[1].composer, "Mozart")
    
    def test_parse_preserves_raw_text(self):
        """Should preserve raw text for debugging"""
        html = """
        <table>
        <tr><td>
        <br>Bach: Brandenburg Concertos 
        <br>Recommended recording: Il Giardino Armonico (1997)
        </td></tr>
        </table>
        """
        
        entries = self.parser.parse_html(html)
        
        self.assertEqual(len(entries), 1)
        self.assertIn("Bach: Brandenburg Concertos", entries[0].raw_text)
        self.assertIn("Il Giardino Armonico", entries[0].raw_text)
    
    def test_parse_real_file(self):
        """Should parse the actual Scaruffi classical.html file"""
        html_path = Path("/mnt/user-data/uploads/classical.html")
        
        if not html_path.exists():
            self.skipTest("classical.html not available")
        
        with open(html_path, 'r', encoding='utf-8') as f:
            html = f.read()
        
        entries = self.parser.parse_html(html)
        
        # Should have many entries (exact count may vary)
        self.assertGreater(len(entries), 200)
        
        # Check a few specific known entries
        bach_entries = [e for e in entries if e.composer == "Bach"]
        self.assertGreater(len(bach_entries), 0)
        
        # Check that entries are well-formed
        for entry in entries[:10]:  # Check first 10
            self.assertIsInstance(entry, ScaruffiEntry)
            self.assertTrue(entry.composer)
            self.assertTrue(entry.work)
            self.assertTrue(entry.raw_text)


if __name__ == '__main__':
    unittest.main()
