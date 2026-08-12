#!/usr/bin/env python3
"""Enforce the Core coverage gate from a `go test -coverprofile` output.

The gate measures product packages only: cmd/autosecrets-core (main wiring)
and internal/testutil (test infrastructure) are excluded because they are
entry points and helpers, not product logic. Usage:

    coverage_gate.py core/coverage.out 70

Exit code 1 (and a summary) when coverage is below the threshold.
"""

from __future__ import annotations

import sys
from pathlib import Path


def parse(profile: Path) -> tuple[int, int]:
    total = 0
    covered = 0
    # Product packages only: entry point and test infrastructure excluded.
    excluded = ("cmd/autosecrets-core", "internal/testutil")
    for line in profile.read_text().splitlines()[1:]:  # skip "mode:" header
        if not line.strip():
            continue
        parts = line.split()
        if len(parts) < 3:
            continue
        if any(part in line for part in excluded):
            continue
        # cover.out block: <file>:<sl.sc,el.ec> <numStmts> <count>
        num_stmts = int(parts[-2])
        count = int(parts[-1])
        total += num_stmts
        if count > 0:
            covered += num_stmts
    return total, covered


def main() -> int:
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} COVER_PROFILE THRESHOLD_PERCENT", file=sys.stderr)
        return 2
    profile = Path(sys.argv[1])
    threshold = float(sys.argv[2])
    if not profile.exists():
        print(f"coverage profile not found: {profile}", file=sys.stderr)
        return 2

    total, covered = parse(profile)
    pct = 100.0 * covered / total if total else 0.0
    print(f"core coverage: {pct:.1f}% ({covered}/{total} statements; "
          f"threshold {threshold:.0f}%)")
    if pct < threshold:
        print(f"FAIL: coverage below {threshold:.0f}%", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
