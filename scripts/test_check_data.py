"""Tests for check-data.py: python -m unittest discover -s scripts -p 'test_*.py'"""

import contextlib
import importlib.util
import io
import json
import os
import sqlite3
import sys
import tempfile
import unittest

# check-data.py is a script (its name is not importable): load it by path, registered so dataclasses can resolve it.
_spec = importlib.util.spec_from_file_location("check_data", os.path.join(os.path.dirname(__file__), "check-data.py"))
check_data = importlib.util.module_from_spec(_spec)
sys.modules["check_data"] = check_data
_spec.loader.exec_module(check_data)


def make_db(path, hospitals, atms=10, bridges=200, line3_km=32.9):
    db = sqlite3.connect(path)
    db.executescript("""
        CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT);
        CREATE TABLE places (id INTEGER PRIMARY KEY, category TEXT, osm_type TEXT);
        CREATE TABLE bridges (id INTEGER PRIMARY KEY, kind TEXT, length_m REAL);
        CREATE TABLE transit_lines (id INTEGER PRIMARY KEY, line_key TEXT, length_m REAL, stop_count INTEGER);
    """)
    rows = [("hospital", "node")] * hospitals + [("atm", "overture")] * atms
    db.executemany("INSERT INTO places (category, osm_type) VALUES (?, ?)", rows)
    db.executemany("INSERT INTO bridges (kind, length_m) VALUES ('road', 100)", [()] * bridges)
    db.execute("INSERT INTO transit_lines (line_key, length_m, stop_count) VALUES ('Mumbai Metro:3', ?, 27)", (line3_km * 1000,))
    db.execute("INSERT INTO meta VALUES ('routing_vertices', '170000')")
    db.commit()
    db.close()


class CheckDataTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.dir.cleanup)
        self.old = os.path.join(self.dir.name, "old.db")
        self.new = os.path.join(self.dir.name, "new.db")
        make_db(self.old, hospitals=1000)

    def run_check(self, *args):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = check_data.main([self.new, "--baseline", self.old, *args])
        return code, out.getvalue()

    def test_collects_the_metrics(self):
        with contextlib.closing(check_data.connect(self.old)) as db:
            stats = check_data.collect(db)
        self.assertEqual(stats["places:hospital"], 1000)
        self.assertEqual(stats["places_source:overture"], 10)
        self.assertEqual(stats["rows:bridges"], 200)
        self.assertEqual(stats["bridges:km"], 20)
        self.assertAlmostEqual(stats["transit:line3_km"], 32.9)
        self.assertEqual(stats["routing:vertices"], 170000)
        self.assertNotIn("rows:buildings", stats)  # missing tables are skipped, not errors

    def test_small_changes_pass(self):
        make_db(self.new, hospitals=970)  # -3 %
        code, out = self.run_check()
        self.assertEqual(code, 0, out)
        self.assertIn("No metric dropped", out)

    def test_large_drop_fails(self):
        make_db(self.new, hospitals=900, line3_km=12.0)
        code, out = self.run_check()
        self.assertEqual(code, 1)
        self.assertIn("`places:hospital` | 1,000 | 900 | -10.0%", out)
        self.assertIn("transit:line3_km", out)
        self.assertEqual(self.run_check("--warn-only")[0], 0)
        self.assertEqual(self.run_check("--threshold", "0.7")[0], 0)

    def test_small_categories_only_warn(self):
        make_db(self.new, hospitals=1000, atms=2)  # -80 % of 10 ATMs
        code, out = self.run_check()
        self.assertEqual(code, 0, out)
        self.assertIn("Smaller drops", out)
        self.assertIn("places:atm", out)

    def test_stats_file_as_baseline(self):
        make_db(self.new, hospitals=500)
        stats = os.path.join(self.dir.name, "stats.json")
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(check_data.main([self.old, "--write-stats", stats]), 0)
        with open(stats, encoding="utf-8") as f:
            self.assertEqual(json.load(f)["places:hospital"], 1000)
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(check_data.main([self.new, "--baseline-stats", stats]), 1)

    def test_missing_file(self):
        with contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(check_data.main([os.path.join(self.dir.name, "nope.db")]), 2)


if __name__ == "__main__":
    unittest.main()
