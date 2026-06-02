#!/usr/bin/env python3
from __future__ import annotations

import base64
import os
import secrets
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any

import requests
from kubernetes import client, config
from kubernetes.client import V1ObjectMeta, V1Secret
from kubernetes.client.exceptions import ApiException


REQUEST_TIMEOUT = 15


@dataclass(frozen=True)
class Settings:
    pod_namespace: str
    pocket_id_internal_url: str
    pocket_id_deployment: str
    pocket_id_bootstrap_secret: str
    pomerium_namespace: str
    pomerium_secret: str
    pomerium_client_id: str
    pomerium_callback_url: str
    pomerium_launch_url: str


def main() -> None:
    settings = load_settings()
    config.load_incluster_config()
    core_api = client.CoreV1Api()
    apps_api = client.AppsV1Api()

    if pomerium_secret_ready(core_api, settings):
        cleanup_static_api_key(core_api, apps_api, settings)
        print("Pomerium OIDC client secret already exists")
        return

    api_key = ensure_static_api_key(core_api, settings)
    restart_pocket_id(apps_api, settings)
    upsert_pomerium_client(settings, api_key)
    write_pomerium_secret(core_api, settings, api_key)
    cleanup_static_api_key(core_api, apps_api, settings)
    print("Pomerium OIDC client bootstrap complete")


def load_settings() -> Settings:
    return Settings(
        pod_namespace=required_env("POD_NAMESPACE"),
        pocket_id_internal_url=required_env("POCKET_ID_INTERNAL_URL").rstrip("/"),
        pocket_id_deployment=required_env("POCKET_ID_DEPLOYMENT"),
        pocket_id_bootstrap_secret=required_env("POCKET_ID_BOOTSTRAP_SECRET"),
        pomerium_namespace=required_env("POMERIUM_NAMESPACE"),
        pomerium_secret=required_env("POMERIUM_SECRET"),
        pomerium_client_id=required_env("POMERIUM_CLIENT_ID"),
        pomerium_callback_url=required_env("POMERIUM_CALLBACK_URL"),
        pomerium_launch_url=required_env("POMERIUM_LAUNCH_URL"),
    )


def required_env(name: str) -> str:
    value = os.environ.get(name)
    if value is None or value == "":
        raise RuntimeError(f"missing required environment variable: {name}")
    return value


def pomerium_secret_ready(core_api: client.CoreV1Api, settings: Settings) -> bool:
    secret = read_secret(core_api, settings.pomerium_namespace, settings.pomerium_secret)
    if secret is None or secret.data is None:
        return False
    return secret_key_exists(secret, "client_id") and secret_key_exists(secret, "client_secret")


def secret_key_exists(secret: V1Secret, key: str) -> bool:
    value = (secret.data or {}).get(key)
    return isinstance(value, str) and value != ""


def cleanup_static_api_key(
    core_api: client.CoreV1Api,
    apps_api: client.AppsV1Api,
    settings: Settings,
) -> None:
    if read_secret(core_api, settings.pod_namespace, settings.pocket_id_bootstrap_secret) is None:
        return

    core_api.delete_namespaced_secret(
        name=settings.pocket_id_bootstrap_secret,
        namespace=settings.pod_namespace,
    )
    restart_pocket_id(apps_api, settings)


def ensure_static_api_key(core_api: client.CoreV1Api, settings: Settings) -> str:
    secret = read_secret(core_api, settings.pod_namespace, settings.pocket_id_bootstrap_secret)
    if secret is not None and secret.data is not None:
        encoded_api_key = secret.data.get("apiKey")
        if encoded_api_key:
            return base64.b64decode(encoded_api_key).decode("utf-8")

    api_key = base64.b64encode(secrets.token_bytes(32)).decode("ascii")
    create_or_patch_secret(
        core_api,
        namespace=settings.pod_namespace,
        name=settings.pocket_id_bootstrap_secret,
        string_data={"apiKey": api_key},
    )
    return api_key


def restart_pocket_id(apps_api: client.AppsV1Api, settings: Settings) -> None:
    deployment = apps_api.read_namespaced_deployment(
        name=settings.pocket_id_deployment,
        namespace=settings.pod_namespace,
    )
    annotations = deployment.spec.template.metadata.annotations
    if annotations is None:
        annotations = {}
    annotations["kubectl.kubernetes.io/restartedAt"] = datetime.now(timezone.utc).isoformat()

    patched = apps_api.patch_namespaced_deployment(
        name=settings.pocket_id_deployment,
        namespace=settings.pod_namespace,
        body={"spec": {"template": {"metadata": {"annotations": annotations}}}},
    )
    wait_for_rollout(apps_api, settings, patched.metadata.generation)
    wait_for_pocket_id(settings)


def wait_for_rollout(
    apps_api: client.AppsV1Api,
    settings: Settings,
    generation: int | None,
) -> None:
    deadline = time.monotonic() + 180
    while time.monotonic() < deadline:
        deployment = apps_api.read_namespaced_deployment(
            name=settings.pocket_id_deployment,
            namespace=settings.pod_namespace,
        )
        desired = deployment.spec.replicas or 0
        status = deployment.status
        if (
            (generation is None or (status.observed_generation or 0) >= generation)
            and (status.updated_replicas or 0) == desired
            and (status.replicas or 0) == desired
            and (status.available_replicas or 0) == desired
            and (status.unavailable_replicas or 0) == 0
        ):
            return
        time.sleep(2)
    raise RuntimeError("timed out waiting for Pocket ID rollout")


def wait_for_pocket_id(settings: Settings) -> None:
    for _ in range(60):
        try:
            response = requests.get(f"{settings.pocket_id_internal_url}/healthz", timeout=REQUEST_TIMEOUT)
            if response.ok:
                return
        except requests.RequestException:
            pass
        time.sleep(2)
    raise RuntimeError("timed out waiting for Pocket ID health endpoint")


def upsert_pomerium_client(settings: Settings, api_key: str) -> None:
    response = requests.get(
        f"{settings.pocket_id_internal_url}/api/oidc/clients/{settings.pomerium_client_id}",
        headers=api_headers(api_key),
        timeout=REQUEST_TIMEOUT,
    )
    payload = client_payload(settings)
    if response.status_code == requests.codes.ok:
        put_response = requests.put(
            f"{settings.pocket_id_internal_url}/api/oidc/clients/{settings.pomerium_client_id}",
            headers=api_headers(api_key),
            json=payload,
            timeout=REQUEST_TIMEOUT,
        )
        put_response.raise_for_status()
        return

    if response.status_code == requests.codes.not_found:
        post_response = requests.post(
            f"{settings.pocket_id_internal_url}/api/oidc/clients",
            headers=api_headers(api_key),
            json=payload,
            timeout=REQUEST_TIMEOUT,
        )
        post_response.raise_for_status()
        return

    print(f"unexpected Pocket ID client lookup status: {response.status_code}", file=sys.stderr)
    print(response.text, file=sys.stderr)
    response.raise_for_status()


def write_pomerium_secret(core_api: client.CoreV1Api, settings: Settings, api_key: str) -> None:
    response = requests.post(
        f"{settings.pocket_id_internal_url}/api/oidc/clients/{settings.pomerium_client_id}/secret",
        headers=api_headers(api_key),
        timeout=REQUEST_TIMEOUT,
    )
    response.raise_for_status()
    client_secret = response.json().get("secret")
    if not isinstance(client_secret, str) or client_secret == "":
        raise RuntimeError("Pocket ID did not return a client secret")

    create_or_patch_secret(
        core_api,
        namespace=settings.pomerium_namespace,
        name=settings.pomerium_secret,
        string_data={
            "client_id": settings.pomerium_client_id,
            "client_secret": client_secret,
        },
    )


def client_payload(settings: Settings) -> dict[str, Any]:
    return {
        "id": settings.pomerium_client_id,
        "name": "Pomerium",
        "callbackURLs": [settings.pomerium_callback_url],
        "logoutCallbackURLs": [],
        "isPublic": False,
        "pkceEnabled": False,
        "requiresReauthentication": False,
        "credentials": {"federatedIdentities": []},
        "launchURL": settings.pomerium_launch_url,
        "isGroupRestricted": False,
    }


def api_headers(api_key: str) -> dict[str, str]:
    return {
        "Content-Type": "application/json",
        "X-API-Key": api_key,
    }


def create_or_patch_secret(
    core_api: client.CoreV1Api,
    *,
    namespace: str,
    name: str,
    string_data: dict[str, str],
) -> None:
    secret = V1Secret(
        metadata=V1ObjectMeta(name=name),
        string_data=string_data,
        type="Opaque",
    )
    try:
        core_api.create_namespaced_secret(namespace=namespace, body=secret)
    except ApiException as exc:
        if exc.status != 409:
            raise
        core_api.patch_namespaced_secret(name=name, namespace=namespace, body=secret)


def read_secret(core_api: client.CoreV1Api, namespace: str, name: str) -> V1Secret | None:
    try:
        return core_api.read_namespaced_secret(name=name, namespace=namespace)
    except ApiException as exc:
        if exc.status == 404:
            return None
        raise


if __name__ == "__main__":
    main()
