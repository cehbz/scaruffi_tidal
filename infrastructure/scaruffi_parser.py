"""
Scaruffi HTML parser.
Infrastructure layer - handles HTML parsing and text extraction.
"""

import re
from typing import Optional
from bs4 import BeautifulSoup

from domain.recording import Recording
from domain.scaruffi_entry import ScaruffiEntry


class ScaruffiParser:
    """Parse Scaruffi's classical music HTML page into structured entries."""
    
    def parse_html(self, html: str) -> list[ScaruffiEntry]:
        """
        Parse HTML content and extract all music entries.
        
        Returns list of ScaruffiEntry objects.
        """
        soup = BeautifulSoup(html, 'html.parser')
        
        # Find the table containing the entries
        table = soup.find('table')
        if not table:
            return []
        
        # Get all text content from table
        text = table.get_text()
        
        # Split by blank lines to separate entries
        entries = []
        current_lines = []
        
        for line in text.split('\n'):
            line = line.strip()
            if not line:
                if current_lines:
                    entry = self._parse_entry_lines(current_lines)
                    if entry:
                        entries.append(entry)
                    current_lines = []
            else:
                current_lines.append(line)
        
        # Handle last entry if exists
        if current_lines:
            entry = self._parse_entry_lines(current_lines)
            if entry:
                entries.append(entry)
        
        return entries
    
    def _parse_entry_lines(self, lines: list[str]) -> Optional[ScaruffiEntry]:
        """
        Parse a single entry from its lines.
        
        Expected format:
        Line 1: "Composer: Work Title"
        Line 2: "Recommended recording: Performer info (metadata)"
        """
        if len(lines) < 2:
            return None
        
        raw_text = '\n'.join(lines)
        
        # Parse first line: "Composer: Work"
        work_line = lines[0]
        if ':' not in work_line:
            return None
        
        composer, work = work_line.split(':', 1)
        composer = composer.strip()
        work = work.strip()
        
        if not composer or not work:
            return None
        
        # Parse second line: "Recommended recording: ..."
        recording_line = lines[1]
        if not recording_line.startswith('Recommended recording:'):
            return None
        
        recording_text = recording_line.replace('Recommended recording:', '').strip()
        
        # Parse primary and alternate recordings
        primary, alternates = self._parse_recordings(composer, work, recording_text)
        
        if not primary:
            return None
        
        return ScaruffiEntry(
            composer=composer,
            work=work,
            primary_recording=primary,
            alternate_recordings=tuple(alternates),
            raw_text=raw_text
        )
    
    def _parse_recordings(
        self, 
        composer: str, 
        work: str, 
        recording_text: str
    ) -> tuple[Optional[Recording], list[Recording]]:
        """
        Parse primary and alternate recordings from recording text.
        
        Handles:
        - "Performer (year)"
        - "Performer (label)"
        - "Performer (year-year)" (year ranges)
        - "Performer (also Alternate1, Alternate2)"
        - "Performer1 or Performer2"
        
        Returns (primary_recording, [alternate_recordings])
        """
        # Check for 'or' separator first (e.g., "Pollini or Zimerman")
        if ' or ' in recording_text and '(also' not in recording_text:
            parts = recording_text.split(' or ')
            primary = self._parse_single_recording(composer, work, parts[0].strip())
            alternates = [
                self._parse_single_recording(composer, work, p.strip())
                for p in parts[1:]
            ]
            alternates = [a for a in alternates if a is not None]
            return primary, alternates
        
        # Split on "(also" to separate primary from alternates
        if '(also' in recording_text:
            parts = recording_text.split('(also', 1)
            primary_text = parts[0].strip()
            alternate_text = parts[1].strip()
            
            # Remove trailing parenthesis from alternates
            if alternate_text.endswith(')'):
                alternate_text = alternate_text[:-1].strip()
            
            primary = self._parse_single_recording(composer, work, primary_text)
            
            # Parse alternates - can be separated by commas or semicolons
            # "Alternate1, Alternate2" or "Alt1; Alt2"
            alternate_parts = re.split(r'[,;]', alternate_text)
            alternates = [
                self._parse_single_recording(composer, work, p.strip())
                for p in alternate_parts
                if p.strip()
            ]
            alternates = [a for a in alternates if a is not None]
            
            return primary, alternates
        else:
            primary = self._parse_single_recording(composer, work, recording_text)
            return primary, []
    
    def _parse_single_recording(
        self, 
        composer: str, 
        work: str, 
        text: str
    ) -> Optional[Recording]:
        """
        Parse a single recording entry.
        
        Formats:
        - "Performer (1997)"
        - "Performer (Label)"
        - "Performer (1997-2000)"
        - "Performer & Orchestra (Label)"
        - "Performer on Label"
        - "Performer"
        """
        if not text:
            return None
        
        performer = None
        year = None
        label = None
        
        # Check for " on " pattern (e.g., "Schiff on ECM")
        if ' on ' in text:
            parts = text.split(' on ')
            performer = parts[0].strip()
            label = parts[1].strip()
            # Remove any parentheses from label
            label = label.strip('()')
        # Check for parentheses
        elif '(' in text and ')' in text:
            # Extract what's in parentheses
            match = re.search(r'\(([^)]+)\)', text)
            if match:
                paren_content = match.group(1).strip()
                performer = text[:match.start()].strip()
                
                # Determine if paren content is year, year range, or label
                # Year: digits only (4 digits)
                # Year range: "1963-73" or "1997-2000" or "1985 & 1988"
                # Label: everything else
                
                year_match = re.match(r'^(\d{4})(?:\s*[-&]\s*\d{2,4})?$', paren_content)
                if year_match:
                    year = int(year_match.group(1))
                else:
                    # It's a label
                    label = paren_content
        else:
            # No parentheses or " on ", just performer name
            performer = text.strip()
        
        if not performer:
            return None
        
        try:
            return Recording(
                composer=composer,
                work=work,
                performer=performer,
                label=label,
                year=year
            )
        except ValueError:
            # Invalid recording, skip it
            return None


if __name__ == '__main__':
    # Quick test
    parser = ScaruffiParser()
    
    test_html = """
    <table>
    <tr><td>
    <br>Bach: Brandenburg Concertos 
    <br>Recommended recording: Il Giardino Armonico (1997)
    </td></tr>
    </table>
    """
    
    entries = parser.parse_html(test_html)
    for entry in entries:
        print(entry)
        print(f"  Performer: {entry.primary_recording.performer}")
        print(f"  Year: {entry.primary_recording.year}")
