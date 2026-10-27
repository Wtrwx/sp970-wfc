import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from tools.configure import parse_node, validate, save_private
from tools.installer import patch_nginx, verify_payload, rollback_script, console_redirect, CONSOLE_ASSETS


class ConfigurationTests(unittest.TestCase):
    def test_import_preserves_auth_sni_and_real_certificate_pin(self):
        pin = "ab" * 32
        node, ip = parse_node(f"hy2://a%3Ab%40c@192.0.2.10:443?insecure=1&sni=uk.example.com&pinSHA256={pin}")
        self.assertEqual(node["auth"], "a:b@c")
        self.assertEqual(node["tls"]["sni"], "uk.example.com")
        self.assertEqual(node["tls"]["pinSHA256"], pin)
        self.assertEqual(node["tun"]["mtu"], 1500)
        validate({"mode": "hysteria2", "proxy_ipv4": ip, "hysteria": node})

    def test_insecure_without_pin_and_unrecognized_fingerprint_are_rejected(self):
        for uri in ["hy2://example@192.0.2.1?insecure=1", "hy2://example@192.0.2.1?fingerprint=chrome", "http://example.com"]:
            with self.subTest(uri=uri), self.assertRaises(ValueError):
                parse_node(uri)

    def test_proxy_cannot_expose_lan_listener_or_execute_external_command(self):
        node, ip = parse_node("hy2://test-only@192.0.2.1:443")
        config = {"mode": "hysteria2", "proxy_ipv4": ip, "hysteria": node}
        node["http"]["listen"] = "0.0.0.0:17891"
        with self.assertRaises(ValueError): validate(config)
        node["http"]["listen"] = "127.0.0.1:17891"
        node["masquerade"] = {"command": "unexpected"}
        with self.assertRaises(ValueError): validate(config)

    def test_direct_mode_does_not_require_or_generate_proxy_credentials(self):
        self.assertEqual(validate({"mode": "direct"}), {"mode": "direct"})

    def test_local_secret_file_is_private(self):
        with tempfile.TemporaryDirectory() as t:
            file = Path(t) / "config.json"
            save_private(file, {"example": "local-only"})
            self.assertEqual(file.stat().st_mode & 0o777, 0o600)


class NativeIntegrationTests(unittest.TestCase):
    def test_rollback_restores_prior_enabled_state_without_deleting_private_data(self):
        for enabled in (True, False):
            script = rollback_script({"/system/var/www/login.html": "login.html"}, "/backup",
                                     old_control=True, old_enabled=enabled)
            self.assertIn('elif [ -f /backup/login.html ]', script)
            self.assertIn('"$BB" cp -p /backup/login.html /system/var/www/login.html', script)
            self.assertNotIn("rm -rf", script)
            self.assertNotIn("forward/", script)
            self.assertTrue(script.rstrip().endswith("enable" if enabled else "disable"))
            self.assertLess(script.index('kill -HUP'), script.rindex('service-control.sh'))
            self.assertIn('"$E" = /system/bin/nginx', script)

    def test_clean_install_rollback_removes_only_new_boot_entry(self):
        script = rollback_script({}, "/backup", old_control=False, old_enabled=False)
        self.assertTrue(script.rstrip().endswith("rm -f /data/adb/service.d/90-sp970-wfc.sh"))
        self.assertNotIn("service-control.sh enable", script)

    def test_nginx_patch_preserves_native_routes_and_is_idempotent(self):
        original = "http { server {\n location / { root /system/var/www; }\n location /sl4a { proxy_pass http://127.0.0.1:8080; }\n} }\n"
        patched = patch_nginx(original)
        self.assertEqual(patch_nginx(patched), patched)
        self.assertEqual(patched.count("location /wfc/"), 1)
        self.assertIn("location /sl4a", patched)
        self.assertIn("client_max_body_size 64k", patched)

    def test_management_upgrade_keeps_workers_unprivileged_and_adds_one_route(self):
        original = "master_process off;\nhttp {server {\n# SP970-WFC managed API\nlocation /wfc/ {proxy_pass http://127.0.0.1:17892/;}\nlocation / {root /system/var/www;}\n}}\n"
        patched = patch_nginx(original)
        self.assertIn("master_process on;", patched)
        self.assertIn("user nobody inet;", patched)
        self.assertNotIn("user root;", patched)
        self.assertEqual(patched.count("location /wfc-admin/"), 1)
        self.assertEqual(patch_nginx(patched), patched)

    def test_console_owns_home_and_login_and_legacy_urls_share_the_same_app(self):
        root = Path(__file__).resolve().parents[1]
        self.assertIn("index.html", CONSOLE_ASSETS)
        self.assertIn("login.html", CONSOLE_ASSETS)
        for name in ("index.html", "login.html"):
            text = (root / "web" / name).read_text()
            self.assertNotIn("<iframe", text)
            self.assertNotIn("common.js", text)
            self.assertIn("/app.js", text)
        self.assertIn('/index.html#sms', console_redirect("sms"))
        with self.assertRaises(ValueError): console_redirect('";unexpected()')

    def test_payload_rejects_tampering_and_directory_traversal(self):
        with tempfile.TemporaryDirectory() as t:
            p = Path(t);(p / "example").write_text("first")
            (p / "SHA256SUMS").write_text(hashlib.sha256(b"first").hexdigest() + "  example\n")
            (p / "example").write_text("changed")
            with self.assertRaisesRegex(ValueError, "校验失败"): verify_payload(p)
            (p / "SHA256SUMS").write_text("a" * 64 + "  ../outside\n")
            with self.assertRaisesRegex(ValueError, "清单格式"): verify_payload(p)


if __name__ == "__main__":
    unittest.main()
