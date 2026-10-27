#!/usr/bin/env python3
"""Authenticated SMS queue that publishes notifications through ntfy.sh/FCM."""

from __future__ import annotations

import hashlib
import hmac
import json
import logging
import os
import random
import signal
import socket
import sqlite3
import ssl
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any


LOG = logging.getLogger("sms-fcm-gateway")


def _env_int(name: str, default: int, minimum: int, maximum: int) -> int:
    raw = os.getenv(name, str(default))
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be an integer") from exc
    if not minimum <= value <= maximum:
        raise ValueError(f"{name} must be between {minimum} and {maximum}")
    return value


@dataclass(frozen=True)
class Config:
    listen_host: str
    listen_port: int
    database_path: Path
    ingress_token: str
    ntfy_base_url: str
    ntfy_topic: str
    ntfy_token: str | None
    device_name: str
    ntfy_timeout_seconds: int
    max_delivery_attempts: int
    force_ipv4: bool

    @classmethod
    def from_environment(cls) -> "Config":
        ingress_token = os.environ["INGRESS_TOKEN"].strip()
        ntfy_topic = os.environ["NTFY_TOPIC"].strip()
        ntfy_base_url = os.getenv("NTFY_BASE_URL", "https://ntfy.sh").strip().rstrip("/")
        parsed_url = urllib.parse.urlparse(ntfy_base_url)

        if len(ingress_token) < 32:
            raise ValueError("INGRESS_TOKEN must contain at least 32 characters")
        if not 20 <= len(ntfy_topic) <= 128 or not all(
            character.isalnum() or character in "-_" for character in ntfy_topic
        ):
            raise ValueError("NTFY_TOPIC must be 20-128 URL-safe characters")
        if parsed_url.scheme != "https" or not parsed_url.netloc or parsed_url.path not in ("", "/"):
            raise ValueError("NTFY_BASE_URL must be an HTTPS origin without a path")

        return cls(
            listen_host=os.getenv("LISTEN_HOST", "127.0.0.1"),
            listen_port=_env_int("LISTEN_PORT", 2587, 1, 65535),
            database_path=Path(
                os.getenv("DATABASE_PATH", "/var/lib/sms-fcm-gateway/queue.sqlite3")
            ),
            ingress_token=ingress_token,
            ntfy_base_url=ntfy_base_url,
            ntfy_topic=ntfy_topic,
            ntfy_token=os.getenv("NTFY_TOKEN", "").strip() or None,
            device_name=os.getenv("DEVICE_NAME", "SP970-B").strip() or "SP970-B",
            ntfy_timeout_seconds=_env_int("NTFY_TIMEOUT_SECONDS", 8, 2, 30),
            max_delivery_attempts=_env_int("MAX_DELIVERY_ATTEMPTS", 30, 1, 100),
            force_ipv4=os.getenv("FORCE_IPV4", "false").strip().lower()
            in {"1", "true", "yes", "on"},
        )


@dataclass(frozen=True)
class SmsEvent:
    id: int
    sender: str
    sms_time: str
    body: str
    attempts: int


def decode_ufi_json_fragment(value: str) -> str:
    """Undo JSONObject.quote escaping added by UFI-TOOLS before placeholder replacement."""
    try:
        decoded = json.loads(f'"{value}"')
    except (json.JSONDecodeError, TypeError):
        return value
    return decoded if isinstance(decoded, str) else value


def truncate_utf8(value: str, byte_limit: int) -> str:
    encoded = value.encode("utf-8")
    if len(encoded) <= byte_limit:
        return value
    suffix = "…"
    budget = byte_limit - len(suffix.encode("utf-8"))
    return encoded[:budget].decode("utf-8", errors="ignore") + suffix


def one_line(value: str, fallback: str) -> str:
    normalized = " ".join(value.replace("\x00", "").split())
    return normalized or fallback


def install_ipv4_only_resolver() -> None:
    """Keep broken host IPv6 routes from delaying every ntfy HTTPS request."""
    original_getaddrinfo = socket.getaddrinfo

    def ipv4_getaddrinfo(
        host: str | bytes | None,
        port: str | int | None,
        family: int = 0,
        type_: int = 0,
        proto: int = 0,
        flags: int = 0,
    ) -> list[tuple[Any, ...]]:
        selected_family = socket.AF_INET if family == socket.AF_UNSPEC else family
        return original_getaddrinfo(host, port, selected_family, type_, proto, flags)

    socket.getaddrinfo = ipv4_getaddrinfo


class QueueStore:
    def __init__(self, database_path: Path):
        self.database_path = database_path
        self.database_path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        self._initialize()

    def _connect(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.database_path, timeout=5)
        connection.row_factory = sqlite3.Row
        connection.execute("PRAGMA busy_timeout = 5000")
        return connection

    def _initialize(self) -> None:
        with self._connect() as connection:
            connection.execute("PRAGMA journal_mode = WAL")
            connection.execute("PRAGMA synchronous = FULL")
            connection.executescript(
                """
                CREATE TABLE IF NOT EXISTS deliveries (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    dedupe_key TEXT NOT NULL UNIQUE,
                    sender TEXT NOT NULL,
                    sms_time TEXT NOT NULL,
                    body TEXT NOT NULL,
                    status TEXT NOT NULL DEFAULT 'pending',
                    attempts INTEGER NOT NULL DEFAULT 0,
                    next_attempt_at INTEGER NOT NULL,
                    created_at INTEGER NOT NULL,
                    sent_at INTEGER,
                    ntfy_message_id TEXT,
                    last_error TEXT
                );
                CREATE INDEX IF NOT EXISTS deliveries_due_idx
                    ON deliveries(status, next_attempt_at, id);
                """
            )
            connection.execute(
                "UPDATE deliveries SET status = 'retry' WHERE status = 'sending'"
            )

    @staticmethod
    def _dedupe_key(sender: str, sms_time: str, body: str) -> str:
        digest = hashlib.sha256()
        for value in (sender, sms_time, body):
            digest.update(value.encode("utf-8"))
            digest.update(b"\x00")
        return digest.hexdigest()

    def enqueue(self, sender: str, sms_time: str, body: str) -> tuple[int, bool]:
        now = int(time.time())
        dedupe_key = self._dedupe_key(sender, sms_time, body)
        with self._connect() as connection:
            cursor = connection.execute(
                """
                INSERT OR IGNORE INTO deliveries
                    (dedupe_key, sender, sms_time, body, next_attempt_at, created_at)
                VALUES (?, ?, ?, ?, ?, ?)
                """,
                (dedupe_key, sender, sms_time, body, now, now),
            )
            created = cursor.rowcount == 1
            row = connection.execute(
                "SELECT id FROM deliveries WHERE dedupe_key = ?", (dedupe_key,)
            ).fetchone()
        if row is None:
            raise RuntimeError("failed to read queued delivery")
        return int(row["id"]), created

    def claim_next(self) -> SmsEvent | None:
        now = int(time.time())
        with self._connect() as connection:
            connection.execute("BEGIN IMMEDIATE")
            row = connection.execute(
                """
                SELECT id, sender, sms_time, body, attempts
                FROM deliveries
                WHERE status IN ('pending', 'retry') AND next_attempt_at <= ?
                ORDER BY id
                LIMIT 1
                """,
                (now,),
            ).fetchone()
            if row is None:
                return None
            updated = connection.execute(
                """
                UPDATE deliveries SET status = 'sending'
                WHERE id = ? AND status IN ('pending', 'retry')
                """,
                (row["id"],),
            )
            if updated.rowcount != 1:
                return None
        return SmsEvent(
            id=int(row["id"]),
            sender=str(row["sender"]),
            sms_time=str(row["sms_time"]),
            body=str(row["body"]),
            attempts=int(row["attempts"]),
        )

    def mark_sent(self, event_id: int, ntfy_message_id: str | None) -> None:
        with self._connect() as connection:
            connection.execute(
                """
                UPDATE deliveries
                SET status = 'sent', sent_at = ?, ntfy_message_id = ?, last_error = NULL
                WHERE id = ?
                """,
                (int(time.time()), ntfy_message_id, event_id),
            )

    def mark_failed(self, event: SmsEvent, error: str, max_attempts: int) -> None:
        attempts = event.attempts + 1
        terminal = attempts >= max_attempts
        delay = min(3600, (2 ** min(attempts, 11)) + random.randint(0, 10))
        with self._connect() as connection:
            connection.execute(
                """
                UPDATE deliveries
                SET status = ?, attempts = ?, next_attempt_at = ?, last_error = ?
                WHERE id = ?
                """,
                (
                    "dead" if terminal else "retry",
                    attempts,
                    int(time.time()) + delay,
                    error[:500],
                    event.id,
                ),
            )

    def counts(self) -> dict[str, int]:
        counts = {"pending": 0, "retry": 0, "sending": 0, "sent": 0, "dead": 0}
        with self._connect() as connection:
            rows = connection.execute(
                "SELECT status, COUNT(*) AS count FROM deliveries GROUP BY status"
            ).fetchall()
        for row in rows:
            counts[str(row["status"])] = int(row["count"])
        return counts

    def prune(self) -> None:
        now = int(time.time())
        with self._connect() as connection:
            connection.execute(
                "DELETE FROM deliveries WHERE status = 'sent' AND sent_at < ?",
                (now - 7 * 86400,),
            )
            connection.execute(
                "DELETE FROM deliveries WHERE status = 'dead' AND created_at < ?",
                (now - 30 * 86400,),
            )


class NtfyPublisher:
    def __init__(self, config: Config):
        self.config = config
        self.ssl_context = ssl.create_default_context()

    def publish(self, event: SmsEvent) -> str | None:
        sender = one_line(event.sender, "未知号码")
        sms_time = one_line(event.sms_time, "未知时间")
        title = truncate_utf8(f"短信 · {sender}", 180)
        message = truncate_utf8(
            f"{event.body}\n\n来自：{sender}\n时间：{sms_time}\n设备：{self.config.device_name}",
            3500,
        )
        payload: dict[str, Any] = {
            "topic": self.config.ntfy_topic,
            "title": title,
            "message": message,
            "priority": 5,
            "tags": ["incoming_envelope"],
        }
        headers = {
            "Content-Type": "application/json; charset=utf-8",
            "User-Agent": "sms-fcm-gateway/1.0",
        }
        if self.config.ntfy_token:
            headers["Authorization"] = f"Bearer {self.config.ntfy_token}"

        request = urllib.request.Request(
            self.config.ntfy_base_url,
            data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
            headers=headers,
            method="POST",
        )
        try:
            with urllib.request.urlopen(
                request,
                timeout=self.config.ntfy_timeout_seconds,
                context=self.ssl_context,
            ) as response:
                response_body = response.read(8192)
                if not 200 <= response.status < 300:
                    raise RuntimeError(f"ntfy returned HTTP {response.status}")
        except urllib.error.HTTPError as exc:
            detail = exc.read(512).decode("utf-8", errors="replace")
            raise RuntimeError(f"ntfy returned HTTP {exc.code}: {detail}") from exc
        except urllib.error.URLError as exc:
            raise RuntimeError(f"ntfy request failed: {exc.reason}") from exc

        try:
            response_json = json.loads(response_body)
        except json.JSONDecodeError:
            return None
        message_id = response_json.get("id")
        return str(message_id) if message_id else None


class DeliveryWorker(threading.Thread):
    def __init__(self, store: QueueStore, publisher: NtfyPublisher, max_attempts: int):
        super().__init__(name="ntfy-delivery-worker", daemon=True)
        self.store = store
        self.publisher = publisher
        self.max_attempts = max_attempts
        self.stop_event = threading.Event()
        self.last_prune = 0.0

    def stop(self) -> None:
        self.stop_event.set()

    def run(self) -> None:
        while not self.stop_event.is_set():
            if time.monotonic() - self.last_prune > 3600:
                self.store.prune()
                self.last_prune = time.monotonic()

            event = self.store.claim_next()
            if event is None:
                self.stop_event.wait(1)
                continue
            try:
                message_id = self.publisher.publish(event)
            except Exception as exc:  # The queue must survive all publisher failures.
                error = f"{type(exc).__name__}: {exc}"
                self.store.mark_failed(event, error, self.max_attempts)
                LOG.warning("delivery id=%s failed: %s", event.id, error)
            else:
                self.store.mark_sent(event.id, message_id)
                LOG.info("delivery id=%s sent", event.id)


class GatewayHTTPServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, address: tuple[str, int], config: Config, store: QueueStore):
        super().__init__(address, GatewayHandler)
        self.config = config
        self.store = store


class GatewayHandler(BaseHTTPRequestHandler):
    server: GatewayHTTPServer
    server_version = "sms-fcm-gateway/1.0"

    def log_message(self, format_string: str, *args: Any) -> None:
        LOG.info("http client=%s " + format_string, self.client_address[0], *args)

    def _respond_json(self, status: HTTPStatus, payload: dict[str, Any]) -> None:
        body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.end_headers()
        self.wfile.write(body)

    def _authorized(self) -> bool:
        scheme, separator, token = self.headers.get("Authorization", "").partition(" ")
        return bool(
            separator
            and scheme.lower() == "bearer"
            and hmac.compare_digest(token.strip(), self.server.config.ingress_token)
        )

    def _require_authorization(self) -> bool:
        if self._authorized():
            return True
        self._respond_json(HTTPStatus.UNAUTHORIZED, {"error": "unauthorized"})
        return False

    def do_GET(self) -> None:
        path = urllib.parse.urlparse(self.path).path
        if path == "/healthz":
            self._respond_json(
                HTTPStatus.OK,
                {"status": "ok", "queue": self.server.store.counts()},
            )
            return
        if path == "/v1/status":
            if not self._require_authorization():
                return
            self._respond_json(
                HTTPStatus.OK,
                {"status": "ok", "queue": self.server.store.counts()},
            )
            return
        self._respond_json(HTTPStatus.NOT_FOUND, {"error": "not_found"})

    def do_POST(self) -> None:
        path = urllib.parse.urlparse(self.path).path
        if path != "/v1/sms":
            self._respond_json(HTTPStatus.NOT_FOUND, {"error": "not_found"})
            return
        if not self._require_authorization():
            return

        try:
            content_length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            self._respond_json(HTTPStatus.BAD_REQUEST, {"error": "invalid_content_length"})
            return
        if content_length <= 0 or content_length > 32768:
            self._respond_json(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, {"error": "invalid_body_size"})
            return

        raw_body = self.rfile.read(content_length)
        content_type = self.headers.get("Content-Type", "").split(";", 1)[0].strip().lower()
        try:
            fields = self._parse_fields(content_type, raw_body)
            sender = str(fields.get("from", "")).strip()
            sms_time = str(fields.get("time", "")).strip()
            body = str(fields.get("body", ""))
            if str(fields.get("ufi_escaped", "")).lower() in {"1", "true", "yes"}:
                body = decode_ufi_json_fragment(body)
        except (UnicodeDecodeError, json.JSONDecodeError, ValueError) as exc:
            self._respond_json(HTTPStatus.BAD_REQUEST, {"error": "invalid_request", "detail": str(exc)})
            return

        if not body.strip():
            self._respond_json(HTTPStatus.BAD_REQUEST, {"error": "body_required"})
            return
        if len(sender) > 256 or len(sms_time) > 128 or len(body.encode("utf-8")) > 16384:
            self._respond_json(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, {"error": "field_too_large"})
            return

        event_id, created = self.server.store.enqueue(sender, sms_time, body)
        self._respond_json(
            HTTPStatus.ACCEPTED if created else HTTPStatus.OK,
            {"accepted": True, "duplicate": not created, "id": event_id},
        )

    def _parse_fields(self, content_type: str, raw_body: bytes) -> dict[str, Any]:
        decoded = raw_body.decode("utf-8")
        if content_type == "application/json":
            payload = json.loads(decoded)
            if not isinstance(payload, dict):
                raise ValueError("JSON body must be an object")
            return payload
        if content_type in ("application/x-www-form-urlencoded", ""):
            parsed = urllib.parse.parse_qs(decoded, keep_blank_values=True)
            return {key: values[-1] for key, values in parsed.items()}
        if content_type == "text/plain":
            return {
                "body": decoded,
                "from": self.headers.get("X-SMS-From", ""),
                "time": self.headers.get("X-SMS-Time", ""),
                "ufi_escaped": self.headers.get("X-UFI-Escaped", ""),
            }
        raise ValueError("unsupported content type")


def main() -> None:
    logging.basicConfig(
        level=os.getenv("LOG_LEVEL", "INFO").upper(),
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    config = Config.from_environment()
    if config.force_ipv4:
        install_ipv4_only_resolver()
        LOG.info("outbound hostname resolution restricted to IPv4")
    store = QueueStore(config.database_path)
    worker = DeliveryWorker(store, NtfyPublisher(config), config.max_delivery_attempts)
    server = GatewayHTTPServer((config.listen_host, config.listen_port), config, store)

    def stop_server(signum: int, _frame: Any) -> None:
        LOG.info("received signal %s, stopping", signum)
        worker.stop()
        threading.Thread(target=server.shutdown, daemon=True).start()

    signal.signal(signal.SIGTERM, stop_server)
    signal.signal(signal.SIGINT, stop_server)
    worker.start()
    LOG.info("listening on %s:%s", config.listen_host, config.listen_port)
    try:
        server.serve_forever(poll_interval=0.5)
    finally:
        worker.stop()
        worker.join(timeout=10)
        server.server_close()


if __name__ == "__main__":
    main()
