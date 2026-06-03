#!/usr/bin/env python3
import asyncio
import hashlib
import json
import os
import re
import ssl
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

import websockets

CERT_PATH = Path("/cert/tls.crt")
KEY_PATH = Path("/cert/tls.key")
API_KEY_PATH = Path("/credentials/api-key")
REQUEST_TIMEOUT_SECONDS = 30
JOB_POLL_SECONDS = 2
JOB_TIMEOUT_SECONDS = 180
PEM_CERT_RE = re.compile(
    rb"-----BEGIN CERTIFICATE-----\s+.*?\s+-----END CERTIFICATE-----",
    re.DOTALL,
)


async def main() -> int:
    log("TrueNAS certificate sync starting")
    cert = CERT_PATH.read_text(encoding="utf-8")
    key = KEY_PATH.read_text(encoding="utf-8")
    api_key = API_KEY_PATH.read_text(encoding="utf-8").strip()
    host = json.loads(require_env("TRUENAS_HOST"))
    port = int(os.environ.get("TRUENAS_PORT", "443"))
    certificate_name = require_env("TRUENAS_CERTIFICATE_NAME")
    local_fingerprint = certificate_fingerprint(cert.encode("utf-8"))

    url = f"wss://{host_address(host)}:{port}/api/current"
    log(
        "Loaded sync inputs",
        host=host.get("name"),
        address=host_address(host),
        port=port,
        certificate_name=certificate_name,
        local_fingerprint=fingerprint_prefix(local_fingerprint),
    )
    context = ssl.create_default_context()
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE

    log("Connecting to TrueNAS API", url=url)
    async with websockets.connect(
        url,
        ssl=context,
        open_timeout=REQUEST_TIMEOUT_SECONDS,
        close_timeout=REQUEST_TIMEOUT_SECONDS,
    ) as websocket:
        client = TrueNasClient(websocket)
        log("Authenticating with TrueNAS API")
        await client.call("auth.login_with_api_key", [api_key])
        log("Authenticated with TrueNAS API")
        await sync_certificate(client, certificate_name, cert, key, local_fingerprint)

    log("TrueNAS certificate sync complete")
    return 0


async def sync_certificate(
    client: "TrueNasClient",
    certificate_name: str,
    cert: str,
    key: str,
    local_fingerprint: str,
) -> None:
    log("Reading current UI certificate")
    current = await current_ui_certificate(client)
    current_fingerprint = certificate_object_fingerprint(current)
    if current_fingerprint == local_fingerprint:
        log("UI certificate already current", current_fingerprint=fingerprint_prefix(current_fingerprint))
        return

    log(
        "UI certificate differs",
        current_fingerprint=fingerprint_prefix(current_fingerprint),
        local_fingerprint=fingerprint_prefix(local_fingerprint),
    )
    managed = await certificate_by_name(client, certificate_name)
    if managed is None:
        log("Creating imported certificate", certificate_name=certificate_name)
        job_id = await client.call(
            "certificate.create",
            [
                {
                    "name": certificate_name,
                    "create_type": "CERTIFICATE_CREATE_IMPORTED",
                    "certificate": cert,
                    "privatekey": key,
                }
            ],
        )
        await client.wait_for_job(job_id)
    else:
        log("Updating imported certificate", certificate_name=certificate_name, certificate_id=managed["id"])
        job_id = await client.call("certificate.update", [managed["id"], {"certificate": cert, "privatekey": key}])
        await client.wait_for_job(job_id)

    managed = await certificate_by_name(client, certificate_name)
    if managed is None:
        raise RuntimeError(f"TrueNAS certificate {certificate_name} was not found after create/update")

    config = await client.call("system.general.config")
    selected_id = selected_certificate_id(config.get("ui_certificate"))
    if selected_id != managed["id"]:
        log("Selecting UI certificate", certificate_name=certificate_name, certificate_id=managed["id"], previous_id=selected_id)
        await client.call("system.general.update", [{"ui_certificate": managed["id"], "ui_restart_delay": 5}])
    else:
        log("Managed certificate already selected; restarting UI", certificate_name=certificate_name)
        await client.call("system.general.ui_restart")


async def current_ui_certificate(client: "TrueNasClient") -> dict[str, Any] | None:
    config = await client.call("system.general.config")
    selected = config.get("ui_certificate")
    if isinstance(selected, dict):
        return selected

    selected_id = selected_certificate_id(selected)
    if selected_id is None:
        return None

    matches = await client.call("certificate.query", [[["id", "=", selected_id]]])
    return matches[0] if matches else None


async def certificate_by_name(client: "TrueNasClient", name: str) -> dict[str, Any] | None:
    matches = await client.call("certificate.query", [[["name", "=", name]]])
    return matches[0] if matches else None


class TrueNasClient:
    def __init__(self, websocket: Any):
        self.websocket = websocket
        self.next_id = 1

    async def call(self, method: str, params: list[Any] | None = None) -> Any:
        request_id = self.next_id
        self.next_id += 1
        log("Calling TrueNAS API", method=method, request_id=request_id)
        await self.websocket.send(
            json.dumps(
                {
                    "jsonrpc": "2.0",
                    "method": method,
                    "params": params or [],
                    "id": request_id,
                }
            )
        )
        while True:
            response = json.loads(await asyncio.wait_for(self.websocket.recv(), timeout=REQUEST_TIMEOUT_SECONDS))
            if response.get("id") != request_id:
                continue
            if response.get("error") is not None:
                log("TrueNAS API call failed", method=method, request_id=request_id, error=response["error"], stream=sys.stderr)
                raise RuntimeError(f"{method} failed: {response['error']}")
            log("TrueNAS API call complete", method=method, request_id=request_id)
            return response.get("result")

    async def wait_for_job(self, job_id: Any) -> None:
        if not isinstance(job_id, int):
            log("Method returned without job id", result=job_id)
            return

        log("Waiting for TrueNAS job", job_id=job_id, timeout_seconds=JOB_TIMEOUT_SECONDS)
        deadline = asyncio.get_running_loop().time() + JOB_TIMEOUT_SECONDS
        last_logged_state = None
        last_log_time = 0.0
        while True:
            matches = await self.call("core.get_jobs", [[["id", "=", job_id]]])
            job = matches[0] if matches else None
            state = job.get("state") if isinstance(job, dict) else None
            now = asyncio.get_running_loop().time()
            if state != last_logged_state or now - last_log_time >= 10:
                log("TrueNAS job state", job_id=job_id, state=state)
                last_logged_state = state
                last_log_time = now
            if state == "SUCCESS":
                log("TrueNAS job complete", job_id=job_id)
                return
            if state == "FAILED":
                raise RuntimeError(f"TrueNAS job {job_id} failed: {job.get('error')}")
            if asyncio.get_running_loop().time() >= deadline:
                raise TimeoutError(f"Timed out waiting for TrueNAS job {job_id}")
            await asyncio.sleep(JOB_POLL_SECONDS)


def certificate_object_fingerprint(certificate: dict[str, Any] | None) -> str | None:
    if certificate is None:
        return None

    pem = certificate.get("certificate")
    if not isinstance(pem, str) or pem == "":
        return None
    return certificate_fingerprint(pem.encode("utf-8"))


def certificate_fingerprint(pem_bundle: bytes) -> str:
    match = PEM_CERT_RE.search(pem_bundle)
    if match is None:
        raise RuntimeError("certificate bundle does not contain a PEM certificate")

    der = ssl.PEM_cert_to_DER_cert(match.group(0).decode("ascii"))
    return hashlib.sha256(der).hexdigest()


def selected_certificate_id(selected: Any) -> int | None:
    if isinstance(selected, int):
        return selected
    if isinstance(selected, dict) and isinstance(selected.get("id"), int):
        return selected["id"]
    return None


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


def fingerprint_prefix(fingerprint: str | None) -> str | None:
    return fingerprint[:16] if fingerprint is not None else None


def log(message: str, stream: Any = sys.stdout, **fields: Any) -> None:
    timestamp = datetime.now(timezone.utc).isoformat(timespec="seconds")
    suffix = "".join(f" {key}={json.dumps(value, sort_keys=True)}" for key, value in fields.items())
    print(f"{timestamp} {message}{suffix}", file=stream, flush=True)


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
