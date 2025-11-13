"""
Canonical performers, conductors, and labels for quality ranking.

These lists represent well-respected artists and labels in classical music.
Higher weight is given to releases by these canonical sources.
"""

# Canonical conductors (alphabetically sorted)
CANONICAL_CONDUCTORS = frozenset({
    # Historic greats
    "Abbado", "Bernstein", "Böhm", "Boulez", "Furtwängler",
    "Giulini", "Harnoncourt", "Jochum", "Karajan", "Klemperer",
    "Kubelik", "Maazel", "Mehta", "Muti", "Solti", "Szell",
    
    # Period instrument specialists
    "Gardiner", "Herreweghe", "Hogwood", "Leonhardt", "Minkowski",
    "Norrington", "Pinnock", "Savall",
    
    # Contemporary masters
    "Abbado", "Barenboim", "Blomstedt", "Chailly", "Currentzis",
    "Dudamel", "Gergiev", "Haitink", "Jansons", "Rattle",
    "Salonen", "Thielemann",
    
    # Chamber/baroque specialists
    "Koopman", "Parrott", "Rifkin",
})

# Canonical pianists
CANONICAL_PIANISTS = frozenset({
    # Historic
    "Arrau", "Ashkenazy", "Backhaus", "Brendel", "Fischer",
    "Gilels", "Gould", "Horowitz", "Kempff", "Michelangeli",
    "Richter", "Rubinstein", "Serkin", "Schnabel",
    
    # Contemporary
    "Aimard", "Andsnes", "Argerich", "Barenboim", "Pollini",
    "Perahia", "Schiff", "Uchida", "Zimerman",
    
    # Fortepianists
    "Bezuidenhout", "Brautigam", "Hewitt", "Tan",
})

# Canonical string players
CANONICAL_STRING_PLAYERS = frozenset({
    # Violinists
    "Bell", "Grumiaux", "Hahn", "Heifetz", "Menuhin",
    "Mutter", "Oistrakh", "Perlman", "Stern", "Vengerov",
    
    # Cellists
    "Casals", "Du Pré", "Fournier", "Ma", "Rostropovich",
    "Isserlis", "Maisky",
    
    # Violists
    "Bashmet", "Zimmermann",
})

# Canonical chamber ensembles
CANONICAL_ENSEMBLES = frozenset({
    # String quartets
    "Alban Berg Quartet", "Amadeus Quartet", "Artemis Quartet",
    "Borodin Quartet", "Emerson String Quartet", "Guarneri Quartet",
    "Hagen Quartet", "Juilliard String Quartet", "Takács Quartet",
    
    # Period ensembles
    "Academy of Ancient Music", "Academy of St Martin", "Amsterdam Baroque",
    "Collegium Vocale", "English Baroque Soloists", "Europa Galante",
    "Freiburger Barockorchester", "Il Giardino Armonico",
    "Les Arts Florissants", "Musica Antiqua Köln",
    "Orchestra of the Age of Enlightenment", "Taverner Consort",
    
    # Vocal ensembles
    "Hilliard Ensemble", "King's Singers", "Monteverdi Choir",
    "Sixteen", "Tallis Scholars",
})

# Canonical orchestras
CANONICAL_ORCHESTRAS = frozenset({
    # Major symphonic orchestras
    "Berliner Philharmoniker", "Berlin Philharmonic",
    "Wiener Philharmoniker", "Vienna Philharmonic", "Wiener",
    "London Symphony", "Philharmonia",
    "Chicago Symphony", "Boston Symphony",
    "Cleveland Orchestra", "New York Philharmonic",
    "Concertgebouw", "Gewandhaus",
    "Staatskapelle Dresden", "Bavarian Radio",
    
    # Chamber orchestras
    "Chamber Orchestra of Europe", "Mahler Chamber Orchestra",
})

# Canonical labels (focusing on classical music specialists)
CANONICAL_LABELS = frozenset({
    # Historic prestige labels
    "Deutsche Grammophon", "DG", "DGG",
    "Decca", "EMI", "Philips",
    "RCA", "Columbia", "CBS",
    
    # Period instrument specialists
    "Archiv", "Harmonia Mundi", "DHM",
    "Erato", "Virgin Classics",
    
    # Audiophile/quality labels
    "ECM", "Hyperion", "BIS",
    "Chandos", "Naxos",
    "Sony Classical", "Warner Classics",
    
    # Historical reissues
    "Testament", "Pristine",
})

# Combined set of all canonical performers (for quick lookup)
ALL_CANONICAL_PERFORMERS = (
    CANONICAL_CONDUCTORS | 
    CANONICAL_PIANISTS | 
    CANONICAL_STRING_PLAYERS | 
    CANONICAL_ENSEMBLES |
    CANONICAL_ORCHESTRAS
)


def is_canonical_performer(name: str) -> bool:
    """
    Check if a performer is in the canonical list.
    
    Uses substring matching (e.g., "Karajan" matches "Herbert von Karajan").
    """
    if not name:
        return False
    
    name_lower = name.lower()
    
    # Check if any canonical name is a substring of the given name
    # or vice versa (handles "Karajan" vs "Herbert von Karajan")
    return any(
        canonical.lower() in name_lower or name_lower in canonical.lower()
        for canonical in ALL_CANONICAL_PERFORMERS
    )


def is_canonical_label(label: str) -> bool:
    """
    Check if a label is in the canonical list.
    
    Uses substring matching (e.g., "DG" matches "Deutsche Grammophon").
    """
    if not label:
        return False
    
    label_lower = label.lower()
    
    return any(
        canonical.lower() in label_lower or label_lower in canonical.lower()
        for canonical in CANONICAL_LABELS
    )


def get_canonical_performer_score(name: str) -> float:
    """
    Get a quality score for a performer (0.0-1.0).
    
    Returns 1.0 for canonical performers, 0.0 otherwise.
    Could be extended to have tiers of performers.
    """
    return 1.0 if is_canonical_performer(name) else 0.0


def get_canonical_label_score(label: str) -> float:
    """
    Get a quality score for a label (0.0-1.0).
    
    Returns 1.0 for canonical labels, 0.0 otherwise.
    Could be extended to have tiers of labels.
    """
    return 1.0 if is_canonical_label(label) else 0.0
