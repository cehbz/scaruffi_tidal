#!/usr/bin/env python3
"""Dumb HTML → text pre-pass for interpret inputs — the dependency-free FALLBACK.

The primary Pass-0 tool is `defuddle parse <url|file> --md` (see INTERPRET.md):
it extracts main content, so nav/footer junk never reaches the interpreter.
This script is for the fallback cases (defuddle unavailable, or malformed HTML
collapsing its extraction root): strips tags, turns <br>/<p>/<li>/<tr> into
newlines, decodes entities, collapses blank runs. The output is read by the
LLM interpreter, not parsed by machine.
"""

import sys
from html.parser import HTMLParser


class TextExtractor(HTMLParser):
    BREAKERS = {"br", "p", "li", "tr", "div", "h1", "h2", "h3", "h4", "table"}
    SKIP = {"script", "style"}

    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.parts: list[str] = []
        self._skip_depth = 0

    def handle_starttag(self, tag, attrs):
        if tag in self.SKIP:
            self._skip_depth += 1
        elif tag in self.BREAKERS:
            self.parts.append("\n")

    def handle_endtag(self, tag):
        if tag in self.SKIP and self._skip_depth:
            self._skip_depth -= 1
        elif tag in self.BREAKERS:
            self.parts.append("\n")

    def handle_data(self, data):
        if not self._skip_depth:
            self.parts.append(data)


def html_to_text(html: str) -> str:
    p = TextExtractor()
    p.feed(html)
    text = "".join(p.parts)
    lines = [" ".join(ln.split()) for ln in text.splitlines()]
    out: list[str] = []
    for ln in lines:
        if ln or (out and out[-1]):
            out.append(ln)
    return "\n".join(out).strip() + "\n"


if __name__ == "__main__":
    src = sys.argv[1] if len(sys.argv) > 1 else "-"
    html = sys.stdin.read() if src == "-" else open(src, encoding="utf-8", errors="replace").read()
    sys.stdout.write(html_to_text(html))
