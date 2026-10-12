#!/usr/bin/env python3
"""Compares a freshly built bombaychitra.db against the previous release and fails on suspicious drops.

A sudden fall in the number of hospitals, bridges or metro lines usually means upstream vandalism in OpenStreetMap,
a broken Overture Maps download or a schema change in either, not a real change in Mumbai. The weekly Data workflow
runs this before publishing so a bad build never replaces a good one.

    scripts/check-data.py data/build/bombaychitra.db --baseline previous/bombaychitra.db
    scripts/check-data.py data/build/bombaychitra.db --write-stats stats.json      # just record the numbers

Only the Python standard library is used. Exit status: 0 ok, 1 regressions found, 2 usage or file errors.
"""

from __future__ import annotations

import argparse
import contextlib
import json
import os
import sqlite3
import sys
from dataclasses import dataclass

# Metrics with fewer than this many items in the baseline are reported but never fail the check: a category of
# twelve ferry terminals legitimately loses one now and then.
MIN_BASELINE = 50


def connect(path: str) -> sqlite3.Connection:
    if not os.path.exists(path):
        raise FileNotFoundError(path)
    return sqlite3.connect(f"file:{path}?mode=ro", uri=True)


def scalar(db: sqlite3.Connection, sql: str, *args: object) -> float | None:
    try:
        row = db.execute(sql, args).fetchone()
    except sqlite3.OperationalError:
        return None  # a table this build does not have
    return None if row is None or row[0] is None else float(row[0])


def collect(db: sqlite3.Connection) -> dict[str, float]:
    """The numbers worth watching, keyed by a stable metric name."""
    stats: dict[str, float] = {}

    def put(name: str, value: float | None) -> None:
        if value is not None:
            stats[name] = value

    for table in ("places", "localities", "streets", "roads", "bridges", "named_bridges", "buildings", "transit_lines"):
        put(f"rows:{table}", scalar(db, f"SELECT COUNT(*) FROM {table}"))
    try:
        for category, count in db.execute("SELECT category, COUNT(*) FROM places GROUP BY category"):
            stats[f"places:{category}"] = float(count)
        for source, count in db.execute("SELECT osm_type = 'overture', COUNT(*) FROM places GROUP BY 1"):
            stats["places_source:" + ("overture" if source else "openstreetmap")] = float(count)
    except sqlite3.OperationalError:
        pass
    put("bridges:km", scalar(db, "SELECT SUM(length_m) / 1000 FROM bridges WHERE kind <> 'outline'"))
    put("buildings:footprints", scalar(db, "SELECT value FROM area_stats WHERE area_id = '*' AND key = 'buildings'"))
    put("transit:lines", scalar(db, "SELECT COUNT(DISTINCT line_key) FROM transit_lines"))
    put("transit:stops", scalar(db, "SELECT SUM(stop_count) FROM transit_lines"))
    # Line 3 (Aqua Line) is fully underground and mapped in detail: a good canary for transit data.
    put("transit:line3_km", scalar(db, "SELECT MAX(length_m) / 1000 FROM transit_lines WHERE line_key = 'Mumbai Metro:3'"))
    put("areas", scalar(db, "SELECT COUNT(*) FROM areas"))
    for key in ("routing_vertices", "routing_segments", "routing_edges"):
        put(f"routing:{key.removeprefix('routing_')}", scalar(db, "SELECT value FROM meta WHERE key = ?", key))
    return stats


@dataclass
class Finding:
    metric: str
    before: float
    after: float

    @property
    def change(self) -> float:
        return (self.after - self.before) / self.before if self.before else 0.0


def compare(before: dict[str, float], after: dict[str, float], threshold: float) -> tuple[list[Finding], list[Finding]]:
    """Returns (failures, notes): drops beyond the threshold on metrics large enough to matter, and other drops or
    metrics that disappeared."""
    failures: list[Finding] = []
    notes: list[Finding] = []
    for metric, old in sorted(before.items()):
        new = after.get(metric, 0.0)
        if old <= 0:
            continue
        finding = Finding(metric, old, new)
        if finding.change < -threshold:
            (failures if old >= MIN_BASELINE else notes).append(finding)
        elif finding.change < 0 and metric not in after:
            notes.append(finding)
    return failures, notes


def fmt(v: float) -> str:
    return f"{v:,.1f}" if v != int(v) else f"{int(v):,}"


def report(failures: list[Finding], notes: list[Finding], after: dict[str, float], threshold: float) -> str:
    lines = ["## Data regression check", ""]
    if failures:
        lines.append(f"**{len(failures)} metric(s) dropped by more than {threshold:.0%}** — check OpenStreetMap and "
                     "Overture Maps before publishing this build.")
    else:
        lines.append(f"No metric dropped by more than {threshold:.0%}.")
    for title, rows in (("Regressions", failures), ("Smaller drops (not failing)", notes)):
        if rows:
            lines += ["", f"### {title}", "", "| Metric | Before | After | Change |", "|---|---:|---:|---:|"]
            lines += [f"| `{f.metric}` | {fmt(f.before)} | {fmt(f.after)} | {f.change:+.1%} |" for f in rows]
    headline = ["rows:places", "rows:bridges", "buildings:footprints", "transit:lines", "transit:line3_km", "routing:vertices"]
    lines += ["", "### Headline numbers", "", "| Metric | Value |", "|---|---:|"]
    lines += [f"| `{m}` | {fmt(after[m])} |" for m in headline if m in after]
    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("database", help="the new bombaychitra.db")
    parser.add_argument("--baseline", help="the previous bombaychitra.db to compare against")
    parser.add_argument("--baseline-stats", help="or a stats JSON written earlier with --write-stats")
    parser.add_argument("--threshold", type=float, default=0.05, help="fail on drops larger than this (default 0.05)")
    parser.add_argument("--write-stats", help="write the new build's numbers to this JSON file")
    parser.add_argument("--warn-only", action="store_true", help="report regressions but exit 0")
    args = parser.parse_args(argv)

    try:
        with contextlib.closing(connect(args.database)) as db:
            after = collect(db)
        before = None
        if args.baseline:
            with contextlib.closing(connect(args.baseline)) as db:
                before = collect(db)
        elif args.baseline_stats:
            with open(args.baseline_stats, encoding="utf-8") as f:
                before = json.load(f)
    except (FileNotFoundError, sqlite3.DatabaseError, json.JSONDecodeError) as e:
        print(f"error: {e}", file=sys.stderr)
        return 2

    if args.write_stats:
        with open(args.write_stats, "w", encoding="utf-8") as f:
            json.dump(after, f, indent=1, sort_keys=True)
    if before is None:
        print(f"{len(after)} metrics collected; no baseline to compare against")
        return 0

    failures, notes = compare(before, after, args.threshold)
    text = report(failures, notes, after, args.threshold)
    print(text)
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a", encoding="utf-8") as f:
            f.write(text)
    return 1 if failures and not args.warn_only else 0


if __name__ == "__main__":
    sys.exit(main())
