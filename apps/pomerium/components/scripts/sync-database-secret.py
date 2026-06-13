#!/usr/bin/env python3
from __future__ import annotations

import base64
import os
import sys
import time

from kubernetes import client, config
from kubernetes.client import V1Secret
from kubernetes.client.exceptions import ApiException


WAIT_SECONDS = 300
POLL_SECONDS = 5


def main() -> None:
    namespace = required_env("POD_NAMESPACE")
    secret_name = required_env("POMERIUM_DATABASE_SECRET")

    config.load_incluster_config()
    core_api = client.CoreV1Api()

    secret = wait_for_secret(core_api, namespace, secret_name)
    uri = secret_data(secret, "uri")
    if uri == "":
        raise RuntimeError(f"secret {namespace}/{secret_name} is missing non-empty uri")

    current = secret_data(secret, "connection")
    if current == uri:
        print(f"{namespace}/{secret_name} connection key is current")
        return

    patch = V1Secret(string_data={"connection": uri})
    core_api.patch_namespaced_secret(name=secret_name, namespace=namespace, body=patch)
    print(f"{namespace}/{secret_name} connection key synced")


def required_env(name: str) -> str:
    value = os.environ.get(name)
    if value is None or value == "":
        raise RuntimeError(f"missing required environment variable: {name}")
    return value


def wait_for_secret(core_api: client.CoreV1Api, namespace: str, name: str) -> V1Secret:
    deadline = time.monotonic() + WAIT_SECONDS
    while True:
        try:
            return core_api.read_namespaced_secret(name=name, namespace=namespace)
        except ApiException as exc:
            if exc.status != 404:
                raise
            if time.monotonic() >= deadline:
                raise TimeoutError(f"timed out waiting for secret {namespace}/{name}") from exc
            print(f"waiting for secret {namespace}/{name}", file=sys.stderr)
            time.sleep(POLL_SECONDS)


def secret_data(secret: V1Secret, key: str) -> str:
    encoded = (secret.data or {}).get(key)
    if not encoded:
        return ""
    return base64.b64decode(encoded).decode("utf-8")


if __name__ == "__main__":
    main()
