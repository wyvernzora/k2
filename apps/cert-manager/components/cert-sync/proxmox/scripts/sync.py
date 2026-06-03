#!/usr/bin/env python3
import hashlib
import json
import os
import re
import socket
import ssl
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import requests
from urllib3.exceptions import InsecureRequestWarning

CERT_PATH = Path("/cert/tls.crt")
KEY_PATH = Path("/cert/tls.key")
TOKEN_ID_PATH = Path("/credentials/api-token-id")
TOKEN_SECRET_PATH = Path("/credentials/api-token-secret")

REQUEST_TIMEOUT_SECONDS = 30
PEM_CERT_RE = re.compile(
    rb"-----BEGIN CERTIFICATE-----\s+.*?\s+-----END CERTIFICATE-----",
    re.DOTALL,
)


def main() -> int:
    requests.packages.urllib3.disable_warnings(category=InsecureRequestWarning)

    log("Proxmox certificate sync starting")
    cert = CERT_PATH.read_text(encoding="utf-8")
    key = KEY_PATH.read_text(encoding="utf-8")
    local_fingerprint = certificate_fingerprint(cert.encode("utf-8"))
    token_id = TOKEN_ID_PATH.read_text(encoding="utf-8").strip()
    token_secret = TOKEN_SECRET_PATH.read_text(encoding="utf-8").strip()
    hosts = json.loads(require_env("PROXMOX_HOSTS"))
    port = int(os.environ.get("PROXMOX_PORT", "8006"))
    log(
        "Loaded sync inputs",
        hosts=", ".join(f"{host_name(host)}={host_address(host)}" for host in hosts),
        port=port,
        local_fingerprint=fingerprint_prefix(local_fingerprint),
    )

    changed = 0
    skipped = 0
    failures = []
    for host in hosts:
        try:
            if sync_host(host, port, cert, key, local_fingerprint, token_id, token_secret):
                changed += 1
            else:
                skipped += 1
        except Exception as exc:
            log("Host sync failed", host=host_name(host), error=str(exc), stream=sys.stderr)
            failures.append(f"{host_name(host)}: {exc}")

    if failures:
        log("Proxmox certificate sync failed", changed=changed, skipped=skipped, failed=len(failures), stream=sys.stderr)
        for failure in failures:
            log("Failure detail", detail=failure, stream=sys.stderr)
        return 1

    log("Proxmox certificate sync complete", changed=changed, skipped=skipped, failed=0)
    return 0


def sync_host(
    host: dict[str, Any],
    port: int,
    cert: str,
    key: str,
    local_fingerprint: str,
    token_id: str,
    token_secret: str,
) -> bool:
    name = host_name(host)
    address = host_address(host)
    log("Checking host certificate", host=name, address=address, port=port)
    remote_fingerprint = remote_certificate_fingerprint(address, port)
    log("Fetched remote certificate", host=name, remote_fingerprint=fingerprint_prefix(remote_fingerprint))

    if remote_fingerprint == local_fingerprint:
        log("Certificate already current", host=name)
        return False

    log(
        "Certificate differs; uploading replacement",
        host=name,
        remote_fingerprint=fingerprint_prefix(remote_fingerprint),
        local_fingerprint=fingerprint_prefix(local_fingerprint),
    )
    response = requests.post(
        f"https://{address}:{port}/api2/json/nodes/{name}/certificates/custom",
        headers={"Authorization": f"PVEAPIToken={token_id}={token_secret}"},
        data={
            "certificates": cert,
            "key": key,
            "force": "1",
            "restart": "1",
        },
        timeout=REQUEST_TIMEOUT_SECONDS,
        verify=False,
    )
    if not response.ok:
        raise RuntimeError(f"upload returned HTTP {response.status_code}: {response.text[:500]}")
    log("Upload accepted", host=name, status=response.status_code)
    return True


def remote_certificate_fingerprint(host: str, port: int) -> str:
    context = ssl.create_default_context()
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE

    with socket.create_connection((host, port), timeout=REQUEST_TIMEOUT_SECONDS) as sock:
        with context.wrap_socket(sock, server_hostname=host) as tls:
            cert = tls.getpeercert(binary_form=True)

    if cert is None:
        raise RuntimeError("remote host did not present a certificate")
    return hashlib.sha256(cert).hexdigest()


def certificate_fingerprint(pem_bundle: bytes) -> str:
    match = PEM_CERT_RE.search(pem_bundle)
    if match is None:
        raise RuntimeError("local certificate bundle does not contain a PEM certificate")

    der = ssl.PEM_cert_to_DER_cert(match.group(0).decode("ascii"))
    return hashlib.sha256(der).hexdigest()


def host_name(host: dict[str, Any]) -> str:
    value = host.get("name")
    if not isinstance(value, str) or not value:
        raise RuntimeError(f"host entry is missing name: {host!r}")
    return value


def host_address(host: dict[str, Any]) -> str:
    value = host.get("address")
    if not isinstance(value, str) or not value:
        raise RuntimeError(f"host entry is missing address: {host!r}")
    return value


def require_env(name: str) -> str:
    value = os.environ.get(name)
    if value is None or value == "":
        raise RuntimeError(f"{name} is required")
    return value


def fingerprint_prefix(fingerprint: str) -> str:
    return fingerprint[:16]


def log(message: str, stream: Any = sys.stdout, **fields: Any) -> None:
    timestamp = datetime.now(timezone.utc).isoformat(timespec="seconds")
    suffix = "".join(f" {key}={json.dumps(value, sort_keys=True)}" for key, value in fields.items())
    print(f"{timestamp} {message}{suffix}", file=stream, flush=True)


if __name__ == "__main__":
    raise SystemExit(main())
