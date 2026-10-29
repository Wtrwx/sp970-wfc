#!/usr/bin/env python3
"""Restart one pinned Spot VM and publish a plain sing-box node subscription."""
import argparse
import base64
import fcntl
import http.client
import ipaddress
import json
import os
from pathlib import Path
import socket
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid


def atomic_json(path, value, mode=0o600):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_name(path.name + ".next")
    fd = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, mode)
    os.fchmod(fd, mode)
    with os.fdopen(fd, "w") as stream:
        json.dump(value, stream, separators=(",", ":"), sort_keys=True)
        stream.write("\n")
    os.replace(temp, path)


def read_json(path):
    try:
        return json.loads(Path(path).read_text())
    except FileNotFoundError:
        return {}


def b64url(data):
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def ipv4_connection(address, timeout=socket._GLOBAL_DEFAULT_TIMEOUT, source_address=None):
    # Some hosts advertise IPv6 without usable upstream connectivity. Keep the
    # original HTTPS hostname/certificate checks while dialing its public A records.
    error = None
    for result in socket.getaddrinfo(address[0], address[1], socket.AF_INET, socket.SOCK_STREAM):
        try:
            return socket.create_connection(result[4], timeout, source_address)
        except OSError as failure:
            error = failure
    if error is not None:
        raise error
    raise OSError("Google API IPv4 resolution unavailable")


class IPv4HTTPSConnection(http.client.HTTPSConnection):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._create_connection = ipv4_connection


class IPv4HTTPSHandler(urllib.request.HTTPSHandler):
    def https_open(self, request):
        return self.do_open(IPv4HTTPSConnection, request, context=self._context)


def request_json(url, method="GET", body=None, token=None, form=False):
    headers = {}
    data = None
    if token:
        headers["Authorization"] = "Bearer " + token
    if body is not None:
        data = urllib.parse.urlencode(body).encode() if form else json.dumps(body).encode()
        headers["Content-Type"] = "application/x-www-form-urlencoded" if form else "application/json"
    request = urllib.request.Request(url, data=data, headers=headers, method=method)
    # Use the normal verified HTTPS connection, never environment proxy credentials.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), IPv4HTTPSHandler())
    with opener.open(request, timeout=12) as response:
        return json.load(response)


class Compute:
    def __init__(self, config, credential_file):
        self.config = config
        self.credential_file = credential_file
        self.token = None

    def authenticate(self):
        from cryptography.hazmat.primitives import hashes, serialization
        from cryptography.hazmat.primitives.asymmetric import padding
        credentials = read_json(self.credential_file)
        now = int(time.time())
        federated = credentials.get("type") == "oidc_federation"
        header = {"alg": "RS256", "typ": "JWT", "kid": credentials["private_key_id"]}
        if federated:
            claims = {"iss": credentials["issuer"], "sub": credentials["subject"],
                      "aud": credentials["audience"], "iat": now, "exp": now + 120}
        else:
            claims = {"iss": credentials["client_email"],
                      "scope": "https://www.googleapis.com/auth/compute",
                      "aud": "https://oauth2.googleapis.com/token", "iat": now, "exp": now + 600}
        signing = (b64url(json.dumps(header).encode()) + "." + b64url(json.dumps(claims).encode())).encode()
        key = serialization.load_pem_private_key(credentials["private_key"].encode(), password=None)
        signature = key.sign(signing, padding.PKCS1v15(), hashes.SHA256())
        assertion = signing.decode() + "." + b64url(signature)
        if federated:
            response = request_json("https://sts.googleapis.com/v1/token", "POST", {
                "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange",
                "audience": credentials["provider_audience"],
                "requested_token_type": "urn:ietf:params:oauth:token-type:access_token",
                "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
                "scope": "https://www.googleapis.com/auth/cloud-platform",
                "subject_token": assertion}, form=True)
            url = ("https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/" +
                   urllib.parse.quote(credentials["client_email"], safe="") + ":generateAccessToken")
            short = request_json(url, "POST", {
                "scope": ["https://www.googleapis.com/auth/compute"], "lifetime": "600s"},
                token=response["access_token"])
            self.token = short["accessToken"]
        else:
            response = request_json("https://oauth2.googleapis.com/token", "POST", {
                "grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer",
                "assertion": assertion}, form=True)
            self.token = response["access_token"]

    def call(self, action="", method="GET", request_id=None):
        if self.token is None:
            self.authenticate()
        c = self.config
        path = "/projects/{project}/zones/{zone}/instances/{instance}".format(**c)
        url = "https://compute.googleapis.com/compute/v1" + path + action
        if method == "GET":
            url += "?fields=id,status,scheduling/provisioningModel,networkInterfaces/accessConfigs/natIP"
        elif request_id:
            url += "?requestId=" + request_id
        return request_json(url, method, token=self.token)


def recovery(config, cloud, previous, now):
    """Only get/start are exposed. Never stop, reset, create, resize or delete."""
    instance = cloud.call()
    if str(instance.get("id")) != str(config["instance_id"]):
        raise ValueError("instance identity changed; recovery refused")
    if instance.get("scheduling", {}).get("provisioningModel") != "SPOT":
        raise ValueError("provisioning model changed; recovery refused")
    status = instance.get("status", "UNKNOWN")
    state = dict(previous, checked_at=now, vm_status=status)
    state.pop("ipv4", None)
    if status == "TERMINATED":
        # Persist the request ID before POST so an uncertain response is retried
        # idempotently. Accepted/failed starts wait two minutes before another ID.
        last = previous.get("last_start", {})
        if last and now - last.get("at", 0) < 120:
            if last.get("pending"):
                cloud.call("/start", "POST", last["request_id"])
                state["last_start"] = dict(last, pending=False)
                state["outcome"] = "start_requested"
            else:
                state["outcome"] = "waiting_for_capacity"
            return state, None
        state["last_start"] = {"at": now, "request_id": str(uuid.uuid4()), "pending": True}
        state["outcome"] = "start_pending"
        return state, "start"
    if status != "RUNNING":
        state["outcome"] = "waiting_for_transition"
        return state, None
    addresses = [a.get("natIP") for n in instance.get("networkInterfaces", [])
                 for a in n.get("accessConfigs", []) if a.get("natIP")]
    if len(addresses) != 1:
        raise ValueError("expected exactly one external IPv4 address")
    ip = ipaddress.ip_address(addresses[0])
    if ip.version != 4 or not ip.is_global:
        raise ValueError("invalid public IPv4 address")
    state.update(outcome="running", ipv4=str(ip))
    return state, None


def publish(config, state, proxy_file):
    output = Path(config["feed_file"])
    if state["vm_status"] != "RUNNING" or not state.get("ipv4"):
        output.unlink(missing_ok=True)
        return
    node = read_json(proxy_file)
    if (node.get("method") not in {"aes-128-gcm", "aes-256-gcm", "2022-blake3-aes-128-gcm",
            "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305"}
            or not isinstance(node.get("password"), str) or not node["password"]
            or not isinstance(node.get("port", 443), int) or not 1 <= node.get("port", 443) <= 65535):
        raise ValueError("invalid local Shadowsocks configuration")
    # Normal sing-box JSON; the node password is intentionally part of the
    # subscription. Serve only this file at a private, unguessable URL.
    subscription = {"outbounds": [{"type": "shadowsocks", "tag": node.get("name", "UK-WFC"),
        "server": state["ipv4"], "server_port": node.get("port", 443),
        "method": node["method"], "password": node["password"]}]}
    atomic_json(output, subscription, 0o644)

def run(config, credential_file, proxy_file):
    state_file = Path(config["state_dir"]) / "state.json"
    if (state_file.parent / "paused").exists():
        return
    previous = read_json(state_file)
    now = int(time.time())
    cloud = Compute(config, credential_file)
    state, action = recovery(config, cloud, previous, now)
    if action == "start":
        atomic_json(state_file, state)
        cloud.call("/start", "POST", state["last_start"]["request_id"])
        state["last_start"]["pending"] = False
        state["outcome"] = "start_requested"
    publish(config, state, proxy_file)
    atomic_json(state_file, state)
    before = (previous.get("outcome"), previous.get("ipv4"))
    after = (state.get("outcome"), state.get("ipv4"))
    if before != after:
        print(json.dumps({"event": state["outcome"], "vm_status": state["vm_status"],
                          "ipv4": state.get("ipv4")}), flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", default="/etc/gcp-uk-recovery/config.json")
    args = parser.parse_args()
    config = read_json(args.config)
    state_dir = Path(config["state_dir"])
    state_dir.mkdir(parents=True, exist_ok=True)
    with (state_dir / "lock").open("a+") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        credential_dir = Path(os.environ["CREDENTIALS_DIRECTORY"])
        run(config, credential_dir / "gcp.json", credential_dir / "proxy.json")


if __name__ == "__main__":
    try:
        main()
    except urllib.error.HTTPError as error:
        # Never print OAuth assertions, private keys, response bodies or request headers.
        print(json.dumps({"event": "api_error", "http_status": error.code}), flush=True)
        raise SystemExit(1)
    except Exception as error:
        print(json.dumps({"event": "recovery_error", "type": type(error).__name__}), flush=True)
        raise SystemExit(1)
