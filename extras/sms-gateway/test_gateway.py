import json
import hashlib
import sqlite3
from contextlib import closing
import tempfile
import threading
import unittest
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from unittest.mock import patch

from gateway import Config, GatewayHTTPServer, QueueStore, SmsEvent, NtfyPublisher, decode_ufi_json_fragment


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

    def test_device_name_is_persisted_and_devices_deduplicate_independently(self):
        fields = {"from": "demo", "time": "2026-10-28 12:00:00", "body": "demo", "device": "SP970-B"}
        _, first = self.request("/v1/sms", data=urllib.parse.urlencode(fields).encode())
        _, duplicate = self.request("/v1/sms", data=urllib.parse.urlencode(fields).encode())
        self.assertTrue(duplicate["duplicate"])
        fields["device"] = "W200DS"
        _, second = self.request("/v1/sms", data=urllib.parse.urlencode(fields).encode())
        self.assertNotEqual(first["id"], second["id"])
        reopened = QueueStore(self.config.database_path)
        event = reopened.claim_next()
        self.assertEqual(event.device_name, "SP970-B")
        reopened.mark_sent(event.id, None)
        self.assertEqual(reopened.claim_next().device_name, "W200DS")

    def test_publisher_uses_event_device_and_keeps_legacy_default(self):
        publisher = NtfyPublisher(self.config)
        with patch("gateway.urllib.request.urlopen") as request:
            response = request.return_value.__enter__.return_value
            response.status = 200
            response.read.return_value = b'{"id":"local-test"}'
            publisher.publish(SmsEvent(1, "demo", "now", "demo", 0, "SP970-B"))
            payload = json.loads(request.call_args.args[0].data)
            self.assertIn("设备：SP970-B", payload["message"])
            publisher.publish(SmsEvent(2, "demo", "now", "demo", 0))
            payload = json.loads(request.call_args.args[0].data)
            self.assertIn("设备：test-device", payload["message"])

    def test_legacy_queue_migration_preserves_messages_and_deduplication(self):
        path = Path(self.temporary_directory.name) / "legacy.sqlite3"
        digest = hashlib.sha256(b"demo\x00now\x00legacy\x00").hexdigest()
        with closing(sqlite3.connect(path)) as connection, connection:
            connection.execute("""CREATE TABLE deliveries (
                id INTEGER PRIMARY KEY AUTOINCREMENT, dedupe_key TEXT NOT NULL UNIQUE,
                sender TEXT NOT NULL, sms_time TEXT NOT NULL, body TEXT NOT NULL,
                status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0,
                next_attempt_at INTEGER NOT NULL, created_at INTEGER NOT NULL,
                sent_at INTEGER, ntfy_message_id TEXT, last_error TEXT)""")
            connection.execute("INSERT INTO deliveries (dedupe_key,sender,sms_time,body,next_attempt_at,created_at) VALUES (?,?,?,?,0,0)",
                               (digest, "demo", "now", "legacy"))
        migrated = QueueStore(path)
        _, created = migrated.enqueue("demo", "now", "legacy")
        self.assertFalse(created)
        event = migrated.claim_next()
        self.assertEqual(event.body, "legacy")
        self.assertEqual(event.device_name, "")

    def test_health_does_not_require_token(self):
        status, body = self.request("/healthz", token=False)
        self.assertEqual(status, 200)
        self.assertEqual(body["status"], "ok")


if __name__ == "__main__":
    unittest.main()
