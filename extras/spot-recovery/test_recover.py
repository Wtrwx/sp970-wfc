import importlib.util
import base64
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("spot_recover", Path(__file__).with_name("recover.py"))
recover = importlib.util.module_from_spec(spec)
spec.loader.exec_module(recover)


class FakeCompute:
    def __init__(self, status="RUNNING", identity="pinned", model="SPOT"):
        self.calls = []
        self.instance = {"id": identity, "status": status, "scheduling": {"provisioningModel": model},
                         "networkInterfaces": [{"accessConfigs": [{"natIP": "8.8.8.8"}]}]}

    def call(self, action="", method="GET", request_id=None):
        self.calls.append((action, method, request_id))
        return self.instance if not action else {}


class RecoveryTests(unittest.TestCase):
    def test_api_dial_avoids_unusable_ipv6_and_preserves_hostname(self):
        address = (recover.socket.AF_INET, recover.socket.SOCK_STREAM, 6, "", ("8.8.8.8", 443))
        with patch.object(recover.socket, "getaddrinfo", return_value=[address]) as resolve, patch.object(recover.socket, "create_connection", return_value="socket") as dial:
            connection = recover.IPv4HTTPSConnection("sts.googleapis.com", timeout=12)
            self.assertEqual(connection.host, "sts.googleapis.com")
            self.assertTrue(connection._context.check_hostname)
            self.assertEqual(connection._create_connection((connection.host, 443), 12, None), "socket")
        resolve.assert_called_once_with("sts.googleapis.com", 443, recover.socket.AF_INET, recover.socket.SOCK_STREAM)
        dial.assert_called_once_with(("8.8.8.8", 443), 12, None)

    @unittest.skipUnless(importlib.util.find_spec("cryptography"), "cryptography is installed on the recovery host")
    def test_federation_uses_pinned_identity_and_short_service_account_token(self):
        from cryptography.hazmat.primitives import hashes, serialization
        from cryptography.hazmat.primitives.asymmetric import padding, rsa
        key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        credentials = {"type": "oidc_federation", "private_key_id": "test-key",
            "private_key": key.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()).decode(),
            "issuer": "https://identity.example.invalid", "subject": "recovery-host",
            "audience": "recovery-only", "provider_audience": "//iam.googleapis.com/projects/test/locations/global/workloadIdentityPools/test/providers/test",
            "client_email": "recovery@example.invalid"}
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "identity.json"
            path.write_text(json.dumps(credentials))
            with patch.object(recover, "request_json", side_effect=[{"access_token": "federated-test"}, {"accessToken": "sa-test"}]) as request, patch.object(recover.time, "time", return_value=1000):
                cloud = recover.Compute({}, path)
                cloud.authenticate()
        first, second = request.call_args_list
        self.assertEqual(first.args[0], "https://sts.googleapis.com/v1/token")
        self.assertEqual(first.args[2]["audience"], credentials["provider_audience"])
        jwt = first.args[2]["subject_token"].split(".")
        decoded = lambda part: base64.urlsafe_b64decode(part + "=" * (-len(part) % 4))
        claims = json.loads(decoded(jwt[1]))
        self.assertEqual(claims, {"iss": credentials["issuer"], "sub": credentials["subject"], "aud": "recovery-only", "iat": 1000, "exp": 1120})
        key.public_key().verify(decoded(jwt[2]), (jwt[0] + "." + jwt[1]).encode(), padding.PKCS1v15(), hashes.SHA256())
        self.assertTrue(second.args[0].startswith("https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/"))
        self.assertEqual(second.args[2], {"scope": ["https://www.googleapis.com/auth/compute"], "lifetime": "600s"})
        self.assertEqual(second.kwargs["token"], "federated-test")
        self.assertEqual(cloud.token, "sa-test")

    def test_plain_sing_box_subscription_and_removal_when_stopped(self):
        with tempfile.TemporaryDirectory() as directory:
            directory = Path(directory)
            proxy_file = directory / "proxy.json"
            proxy_file.write_text(json.dumps({"method": "aes-256-gcm", "password": "test-only-password", "name": "UK"}))
            output = directory / "feed.json"
            config = {"feed_file": str(output)}
            recover.publish(config, {"vm_status": "RUNNING", "ipv4": "8.8.8.8"}, proxy_file)
            subscription = json.loads(output.read_text())
            self.assertEqual(subscription, {"outbounds": [{"type": "shadowsocks", "tag": "UK", "server": "8.8.8.8", "server_port": 443, "method": "aes-256-gcm", "password": "test-only-password"}]})
            self.assertEqual(output.stat().st_mode & 0o777, 0o644)
            self.assertNotIn("signature", subscription)
            recover.publish(config, {"vm_status": "TERMINATED"}, proxy_file)
            self.assertFalse(output.exists())

    def test_running_instance_is_only_read(self):
        cloud = FakeCompute()
        state, action = recover.recovery({"instance_id": "pinned"}, cloud, {}, 1000)
        self.assertEqual(state["ipv4"], "8.8.8.8")
        self.assertIsNone(action)
        self.assertEqual(cloud.calls, [("", "GET", None)])

    def test_changed_identity_or_model_cannot_start(self):
        for cloud in (FakeCompute("TERMINATED", identity="replacement"), FakeCompute("TERMINATED", model="STANDARD")):
            with self.assertRaises(ValueError):
                recover.recovery({"instance_id": "pinned"}, cloud, {}, 1000)
            self.assertTrue(all(method == "GET" for _, method, _ in cloud.calls))

    def test_start_is_planned_before_it_is_sent(self):
        cloud = FakeCompute("TERMINATED")
        state, action = recover.recovery({"instance_id": "pinned"}, cloud, {}, 1000)
        self.assertEqual(action, "start")
        self.assertTrue(state["last_start"]["pending"])
        self.assertEqual(len(cloud.calls), 1)

    def test_uncertain_start_reuses_request_id_and_backoff(self):
        cloud = FakeCompute("TERMINATED")
        previous = {"last_start": {"at": 1000, "request_id": "stable-id", "pending": True}}
        state, action = recover.recovery({"instance_id": "pinned"}, cloud, previous, 1020)
        self.assertIsNone(action)
        self.assertEqual(cloud.calls[-1], ("/start", "POST", "stable-id"))
        self.assertFalse(state["last_start"]["pending"])
        cloud.calls.clear()
        state, action = recover.recovery({"instance_id": "pinned"}, cloud, state, 1040)
        self.assertEqual(state["outcome"], "waiting_for_capacity")
        self.assertEqual(cloud.calls, [("", "GET", None)])

    def test_private_ip_is_not_published(self):
        cloud = FakeCompute()
        cloud.instance["networkInterfaces"][0]["accessConfigs"][0]["natIP"] = "127.0.0.1"
        with self.assertRaises(ValueError):
            recover.recovery({"instance_id": "pinned"}, cloud, {}, 1000)


if __name__ == "__main__":
    unittest.main()
