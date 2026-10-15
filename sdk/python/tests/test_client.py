"""Tests of the generated Python client against a stub server (no dataset needed).

    python -m unittest discover -s sdk/python/tests
"""
import json
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from bombaychitra import BombayChitra, BombayChitraError  # noqa: E402


class Stub(BaseHTTPRequestHandler):
    """Answers with what it was asked, or with an error for paths under /api/v1/fail."""

    def respond(self, status, body, content_type="application/json"):
        data = body.encode() if isinstance(body, str) else json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def echo(self, body=None):
        url = urlparse(self.path)
        if url.path == "/api/v1/route" and parse_qs(url.query).get("to") == ["18.5,72.0"]:
            return self.respond(422, {"error": {"status": 422, "code": "no_road", "message": "no drivable road"}})
        if url.path == "/api/v1/meta":
            return self.respond(502, "<html>Bad gateway</html>", "text/html")
        self.respond(200, {"method": self.command, "path": url.path, "query": parse_qs(url.query),
                           "key": self.headers.get("X-API-Key"), "body": body})

    def do_GET(self):  # noqa: N802
        self.echo()

    def do_POST(self):  # noqa: N802
        length = int(self.headers.get("Content-Length", 0))
        self.echo(json.loads(self.rfile.read(length)))

    def log_message(self, *args):
        pass


class ClientTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = HTTPServer(("127.0.0.1", 0), Stub)
        threading.Thread(target=cls.server.serve_forever, daemon=True).start()
        cls.api = BombayChitra(f"http://127.0.0.1:{cls.server.server_port}/", api_key="secret")

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()

    def test_query_parameters(self):
        r = self.api.list_places(category=["hospital", "clinic"], named=True, limit=5, area=None)
        self.assertEqual(r["path"], "/api/v1/places")
        self.assertEqual(r["query"], {"category": ["hospital,clinic"], "named": ["true"], "limit": ["5"]})
        self.assertEqual(r["key"], "secret")

    def test_reserved_words_and_path_ids(self):
        r = self.api.route("19.1197,72.8464", "18.94,72.8355", mode="foot")
        self.assertEqual(r["query"]["from"], ["19.1197,72.8464"])
        self.assertEqual(self.api.get_area("ward-f/n")["path"], "/api/v1/areas/ward-f%2Fn")

    def test_json_bodies(self):
        body = {"sources": [{"lat": 19.1, "lon": 72.85}]}
        r = self.api.matrix(body)
        self.assertEqual((r["method"], r["path"], r["body"]), ("POST", "/api/v1/matrix", body))
        self.assertEqual(self.api.search_batch({"queries": ["CST"]})["body"], {"queries": ["CST"]})

    def test_errors(self):
        with self.assertRaises(BombayChitraError) as ctx:
            self.api.route("19.1197,72.8464", "18.5,72.0")
        self.assertEqual((ctx.exception.status, ctx.exception.code), (422, "no_road"))
        with self.assertRaises(BombayChitraError) as ctx:
            self.api.get_meta()
        self.assertEqual((ctx.exception.status, ctx.exception.code), (502, "http_error"))


if __name__ == "__main__":
    unittest.main()
