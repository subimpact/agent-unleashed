#!/usr/bin/env python3
"""
Antigravity-Unleashed Launch Script
"""
import sys
from pathlib import Path

# Ensure project root is on sys.path
sys.path.insert(0, str(Path(__file__).parent.resolve()))

from src.daemon import main

if __name__ == "__main__":
    main()
