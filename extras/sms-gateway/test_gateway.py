import json
import tempfile
import threading
import unittest
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

from gateway import Config, GatewayHTTPServer, QueueStore, decode_ufi_json_fragment


class GatewayTests(unittest.TestCase):
    def setUp(self):
        self.temporary_directory = tempfile.TemporaryDirectory()
        self.store = QueueStore(Path(self.temporary_directory.name) / "queue.sqlite3")
        self.config = Config(
            listen_host="127.0.0.1",
            listen_port=0,
            database_path=Path(self.temporary_directory.name) / "queue.sqlite3",
            ingress_token="t" * 64,
            ntfy_base_url="https://ntfy.sh",
            ntfy_topic="sms-" + "a" * 48,
            ntfy_token=None,
            device_name="test-device",
            ntfy_timeout_seconds=2,
            max_delivery_attempts=3,
            force_ipv4=False,
        )
        self.server = GatewayHTTPServer(("127.0.0.1", 0), self.config, self.store)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.base_url = f"http://127.0.0.1:{self.server.server_address[1]}"

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=3)
        self.temporary_directory.cleanup()

    def request(self, path, *, data=None, token=True):
        headers = {}
        if token:
            headers["Authorization"] = "Bearer " + self.config.ingress_token
        request = urllib.request.Request(self.base_url + path, data=data, headers=headers)
        with urllib.request.urlopen(request, timeout=2) as response:
            return response.status, json.loads(response.read())

    def test_ufi_escape_decode(self):
        self.assertEqual(
            decode_ufi_json_fragment(r'first line\nsecond \"quoted\" line\\tail'),
            'first line\nsecond "quoted" line\\tail',
        )

    def test_accepts_and_deduplicates_ufi_form(self):
        payload = urllib.parse.urlencode(
            {
                "from": "+447700900123",
                "time": "2026-09-02 12:34:56",
                "body": r'hello\nworld \"quoted\"',
                "ufi_escaped": "1",
            }
        ).encode()
        status, first = self.request("/v1/sms", data=payload)
        duplicate_status, second = self.request("/v1/sms", data=payload)

        self.assertEqual(status, 202)
        self.assertEqual(duplicate_status, 200)
        self.assertFalse(first["duplicate"])
        self.assertTrue(second["duplicate"])
        event = self.store.claim_next()
        self.assertIsNotNone(event)
        self.assertEqual(event.body, 'hello\nworld "quoted"')

    def test_rejects_bad_token(self):
        payload = urllib.parse.urlencode({"body": "hello"}).encode()
        with self.assertRaises(urllib.error.HTTPError) as context:
            self.request("/v1/sms", data=payload, token=False)
        try:
            self.assertEqual(context.exception.code, 401)
        finally:
            context.exception.close()

    def test_health_does_not_require_token(self):
        status, body = self.request("/healthz", token=False)
        self.assertEqual(status, 200)
        self.assertEqual(body["status"], "ok")


if __name__ == "__main__":
    unittest.main()
