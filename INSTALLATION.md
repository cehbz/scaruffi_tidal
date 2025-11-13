# Scaruffi-Tidal Installation & Testing Guide

## Project Structure

Your project directory should look like this:

```
scaruffi_tidal/
├── domain/
│   ├── __init__.py
│   ├── recording.py
│   ├── discogs.py
│   ├── tidal.py
│   ├── scaruffi_entry.py
│   └── canonical.py
├── application/
│   ├── __init__.py
│   ├── quality_ranker.py
│   ├── orchestrator.py
│   └── auth.py              # (existing)
├── infrastructure/
│   ├── __init__.py
│   ├── scaruffi_parser.py
│   ├── discogs_client.py
│   ├── tidal_client.py
│   ├── config.py
│   └── rate_limiter.py
├── tests/
│   ├── __init__.py
│   ├── domain/
│   │   ├── __init__.py
│   │   └── test_canonical.py
│   ├── application/
│   │   ├── __init__.py
│   │   └── test_quality_ranker.py
│   └── infrastructure/
│       ├── __init__.py
│       ├── test_config.py
│       ├── test_discogs_client.py
│       ├── test_rate_limiter.py
│       └── test_scaruffi_parser.py
├── pyproject.toml
├── setup.cfg
├── cli.py                   # (existing, optional)
└── README.md
```

## Installation

### Option 1: Editable Install (Recommended for Development)

```bash
cd ~/projects/scaruffi_tidal

# Install in editable mode
pip install -e .

# Now you can run tests from anywhere
python -m unittest discover -s tests -p "test_*.py"
```

### Option 2: PYTHONPATH Method (Quick Fix)

```bash
cd ~/projects/scaruffi_tidal

# Run tests with PYTHONPATH set
PYTHONPATH=. python -m unittest discover -s tests -p "test_*.py"

# Or export it for your session
export PYTHONPATH="${PWD}"
python -m unittest discover -s tests -p "test_*.py"
```

### Option 3: Add __init__.py at Root (Simpler)

```bash
cd ~/projects/scaruffi_tidal

# Create root __init__.py (makes it a package)
touch __init__.py

# Run tests from parent directory
cd ..
python -m unittest discover -s scaruffi_tidal/tests -p "test_*.py"
```

## Running Tests After Installation

Once installed with `pip install -e .`:

```bash
# All tests
python -m unittest discover -s tests -p "test_*.py"

# Specific test file
python -m unittest tests.domain.test_canonical

# Specific test class
python -m unittest tests.domain.test_canonical.TestCanonicalLists

# Specific test method
python -m unittest tests.domain.test_canonical.TestCanonicalLists.test_recognizes_canonical_conductor

# With verbose output
python -m unittest discover -s tests -p "test_*.py" -v
```

## Expected Output

```
........................................................
----------------------------------------------------------------------
Ran 52 tests in 1.118s

OK
```

## Common Issues

### ModuleNotFoundError

**Problem**: `ModuleNotFoundError: No module named 'domain.tidal'`

**Solutions**:
1. Install package: `pip install -e .`
2. Set PYTHONPATH: `export PYTHONPATH="${PWD}"`
3. Check directory structure matches above

### Import Errors with Existing Code

If you have existing `cli.py` or `application/auth.py` that import differently:

**Option A**: Update imports to absolute
```python
# Old (relative)
from auth import create_application

# New (absolute)
from application.auth import create_application
```

**Option B**: Keep separate entry points
```python
# cli.py stays as-is with relative imports
# scaruffi_tidal.py uses absolute imports
```

## Project Setup from Scratch

If starting fresh:

```bash
# 1. Create directory
mkdir -p ~/projects/scaruffi_tidal
cd ~/projects/scaruffi_tidal

# 2. Extract all phase tarballs
tar xzf scaruffi-tidal-phase1.tar.gz
tar xzf scaruffi-tidal-phase2.tar.gz
tar xzf scaruffi-tidal-phase3.tar.gz

# 3. Add project files
# Copy pyproject.toml and setup.cfg to root

# 4. Install
pip install -e .

# 5. Run tests
python -m unittest discover -s tests -p "test_*.py"
```

## Integration with Existing Auth Code

Your existing auth code in `application/auth.py` and `domain/auth.py` should work fine. Just ensure imports are consistent:

```python
# In your existing cli.py
from application.auth import create_application, get_default_config_path
from domain.auth import AuthenticationError

# These should work after pip install -e .
```

## Verifying Installation

```bash
# Check package is installed
pip list | grep scaruffi-tidal

# Check modules are importable
python -c "from domain.recording import Recording; print('✓ domain.recording')"
python -c "from application.quality_ranker import QualityRanker; print('✓ application.quality_ranker')"
python -c "from infrastructure.discogs_client import DiscogsClient; print('✓ infrastructure.discogs_client')"

# All imports work? You're good!
```

## Quick Fix for Your Current Error

Right now, the fastest fix:

```bash
cd ~/projects/scaruffi_tidal
pip install -e .
python -m unittest discover -s tests -p "test_*.py"
```

This should immediately resolve all the `ModuleNotFoundError` issues.
