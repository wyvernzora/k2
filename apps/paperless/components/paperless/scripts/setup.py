#!/usr/bin/env python3
from __future__ import annotations

import base64
import os
import sys
import time
from dataclasses import dataclass
from typing import Any

import requests
from kubernetes import client, config


MCP_GROUP_PERMISSIONS = [
    "add_document",
    "view_document",
    "change_document",
    "add_tag",
    "view_tag",
    "change_tag",
    "add_correspondent",
    "view_correspondent",
    "change_correspondent",
    "add_documenttype",
    "view_documenttype",
    "change_documenttype",
    "add_storagepath",
    "view_storagepath",
    "change_storagepath",
    "add_customfield",
    "view_customfield",
    "change_customfield",
    "add_savedview",
    "view_savedview",
    "change_savedview",
    "view_uisettings",
    "change_uisettings",
    "view_paperlesstask",
    "change_paperlesstask",
    "add_note",
    "view_note",
    "change_note",
]

REQUEST_TIMEOUT = 15


@dataclass(frozen=True)
class Settings:
    pod_namespace: str
    paperless_internal_url: str
    paperless_setup_user: str
    paperless_setup_email: str
    paperless_admin_password: str
    paperless_legacy_admin_user: str
    paperless_human_user: str
    paperless_human_email: str
    paperless_mcp_user: str
    paperless_mcp_group: str
    paperless_mcp_password: str
    paperless_mcp_token_secret: str


class PaperlessClient:
    def __init__(self, base_url: str, token: str | None = None) -> None:
        self.base_url = base_url.rstrip("/")
        self.session = requests.Session()
        if token is not None:
            self.session.headers["Authorization"] = f"Token {token}"

    def token(self, username: str, password: str) -> str | None:
        response = self.session.post(
            f"{self.base_url}/api/token/",
            json={"username": username, "password": password},
            timeout=REQUEST_TIMEOUT,
        )
        if response.status_code != requests.codes.ok:
            return None
        token = response.json().get("token")
        if not isinstance(token, str) or token == "":
            raise RuntimeError(f"Paperless token endpoint returned no token for {username}")
        return token

    def get(self, path: str, params: dict[str, str] | None = None) -> Any:
        return self.request("GET", path, params=params)

    def post(self, path: str, payload: dict[str, Any]) -> Any:
        return self.request("POST", path, json=payload)

    def patch(self, path: str, payload: dict[str, Any]) -> Any:
        return self.request("PATCH", path, json=payload)

    def request(
        self,
        method: str,
        path: str,
        *,
        params: dict[str, str] | None = None,
        json: dict[str, Any] | None = None,
    ) -> Any:
        response = self.session.request(
            method,
            f"{self.base_url}{path}",
            params=params,
            json=json,
            timeout=REQUEST_TIMEOUT,
        )
        if 200 <= response.status_code < 300:
            if response.text == "":
                return None
            return response.json()

        print(f"unexpected Paperless API status {response.status_code} for {method} {path}", file=sys.stderr)
        print(response.text, file=sys.stderr)
        response.raise_for_status()
        raise AssertionError("unreachable")


def main() -> None:
    log("Loading Paperless setup settings")
    settings = load_settings()
    log(f"Waiting for Paperless at {settings.paperless_internal_url}")
    wait_for_paperless(settings.paperless_internal_url)
    log("Paperless is reachable")

    anonymous = PaperlessClient(settings.paperless_internal_url)
    log("Authenticating Paperless setup user")
    admin_token = anonymous.token(settings.paperless_setup_user, settings.paperless_admin_password)
    if admin_token is None:
        log("Setup user unavailable; trying legacy admin user")
        legacy_token = anonymous.token(settings.paperless_legacy_admin_user, settings.paperless_admin_password)
        if legacy_token is None:
            raise RuntimeError("could not authenticate setup or legacy Paperless admin user")
        legacy_admin = PaperlessClient(settings.paperless_internal_url, legacy_token)
        log("Ensuring Paperless setup admin user")
        ensure_user(
            legacy_admin,
            username=settings.paperless_setup_user,
            email=settings.paperless_setup_email,
            password=settings.paperless_admin_password,
            is_staff=True,
            is_superuser=True,
            groups=[],
            permissions=[],
        )
        admin_token = anonymous.token(settings.paperless_setup_user, settings.paperless_admin_password)
        if admin_token is None:
            raise RuntimeError("created Paperless setup user but could not authenticate it")

    admin = PaperlessClient(settings.paperless_internal_url, admin_token)
    log("Ensuring Paperless human admin user")
    ensure_user(
        admin,
        username=settings.paperless_human_user,
        email=settings.paperless_human_email,
        password=settings.paperless_admin_password,
        is_staff=True,
        is_superuser=True,
        groups=[],
        permissions=[],
    )
    log("Ensuring Paperless MCP group")
    mcp_group_id = ensure_group(admin, settings.paperless_mcp_group, MCP_GROUP_PERMISSIONS)
    log("Ensuring Paperless MCP user")
    ensure_user(
        admin,
        username=settings.paperless_mcp_user,
        email="",
        password=settings.paperless_mcp_password,
        is_staff=False,
        is_superuser=True,
        groups=[mcp_group_id],
        permissions=[],
    )

    mcp_token = anonymous.token(settings.paperless_mcp_user, settings.paperless_mcp_password)
    if mcp_token is None:
        raise RuntimeError("could not authenticate Paperless MCP user")
    log("Ensuring Paperless MCP token secret")
    patch_mcp_token_secret(settings, mcp_token)
    log("Paperless user setup complete")


def log(message: str) -> None:
    print(message, flush=True)


def load_settings() -> Settings:
    return Settings(
        pod_namespace=required_env("POD_NAMESPACE"),
        paperless_internal_url=required_env("PAPERLESS_INTERNAL_URL"),
        paperless_setup_user=required_env("PAPERLESS_SETUP_USER"),
        paperless_setup_email=required_env("PAPERLESS_SETUP_EMAIL"),
        paperless_admin_password=required_env("PAPERLESS_ADMIN_PASSWORD"),
        paperless_legacy_admin_user=required_env("PAPERLESS_LEGACY_ADMIN_USER"),
        paperless_human_user=required_env("PAPERLESS_HUMAN_USER"),
        paperless_human_email=required_env("PAPERLESS_HUMAN_EMAIL"),
        paperless_mcp_user=required_env("PAPERLESS_MCP_USER"),
        paperless_mcp_group=required_env("PAPERLESS_MCP_GROUP"),
        paperless_mcp_password=required_env("PAPERLESS_MCP_PASSWORD"),
        paperless_mcp_token_secret=required_env("PAPERLESS_MCP_TOKEN_SECRET"),
    )


def required_env(name: str) -> str:
    value = os.environ.get(name)
    if value is None or value == "":
        raise RuntimeError(f"missing required environment variable: {name}")
    return value


def wait_for_paperless(base_url: str) -> None:
    for attempt in range(90):
        try:
            response = requests.get(base_url, timeout=REQUEST_TIMEOUT)
            if response.ok:
                return
        except requests.RequestException:
            pass
        if attempt > 0 and attempt % 15 == 0:
            log(f"Still waiting for Paperless after {attempt * 2}s")
        time.sleep(2)
    raise RuntimeError("timed out waiting for Paperless")


def ensure_user(
    paperless: PaperlessClient,
    *,
    username: str,
    email: str,
    password: str,
    is_staff: bool,
    is_superuser: bool,
    groups: list[int],
    permissions: list[str],
) -> None:
    payload = {
        "username": username,
        "email": email,
        "password": password,
        "first_name": "",
        "last_name": "",
        "is_active": True,
        "is_staff": is_staff,
        "is_superuser": is_superuser,
        "groups": groups,
        "user_permissions": permissions,
    }
    user = find_user(paperless, username)
    if user is None:
        paperless.post("/api/users/", payload)
        log(f"Created Paperless user {username}")
        return

    if user_matches(user, payload):
        log(f"Paperless user {username} already configured")
        return

    paperless.patch(f"/api/users/{required_id(user)}/", payload)
    log(f"Updated Paperless user {username}")


def ensure_group(paperless: PaperlessClient, name: str, permissions: list[str]) -> int:
    payload = {"name": name, "permissions": permissions}
    group = find_group(paperless, name)
    if group is None:
        paperless.post("/api/groups/", payload)
        log(f"Created Paperless group {name}")
        group = find_group(paperless, name)
    else:
        if group_matches(group, payload):
            log(f"Paperless group {name} already configured")
            return required_id(group)
        paperless.patch(f"/api/groups/{required_id(group)}/", payload)
        log(f"Updated Paperless group {name}")

    if group is None:
        raise RuntimeError(f"could not resolve Paperless group {name} after upsert")
    return required_id(group)


def find_user(paperless: PaperlessClient, username: str) -> dict[str, Any] | None:
    return first_result(paperless.get("/api/users/", params={"username__iexact": username}))


def find_group(paperless: PaperlessClient, name: str) -> dict[str, Any] | None:
    return first_result(paperless.get("/api/groups/", params={"name__iexact": name}))


def first_result(payload: Any) -> dict[str, Any] | None:
    results = payload.get("results") if isinstance(payload, dict) else payload
    if not isinstance(results, list) or len(results) == 0:
        return None
    item = results[0]
    if not isinstance(item, dict):
        raise RuntimeError(f"Paperless lookup returned an invalid result: {item!r}")
    required_id(item)
    return item


def required_id(item: dict[str, Any]) -> int:
    item_id = item.get("id")
    if not isinstance(item_id, int):
        raise RuntimeError(f"Paperless lookup returned an invalid id: {item_id!r}")
    return item_id


def user_matches(user: dict[str, Any], payload: dict[str, Any]) -> bool:
    keys = [
        "username",
        "email",
        "first_name",
        "last_name",
        "is_active",
        "is_staff",
        "is_superuser",
    ]
    return all(user.get(key) == payload[key] for key in keys) and list_matches(
        user.get("groups"), payload["groups"]
    ) and list_matches(user.get("user_permissions"), payload["user_permissions"])


def group_matches(group: dict[str, Any], payload: dict[str, Any]) -> bool:
    return group.get("name") == payload["name"] and list_matches(
        group.get("permissions"), payload["permissions"]
    )


def list_matches(actual: Any, expected: list[Any]) -> bool:
    if not isinstance(actual, list):
        return False
    return normalized_list(actual) == normalized_list(expected)


def normalized_list(items: list[Any]) -> list[str]:
    return sorted(str(item) for item in items)


def patch_mcp_token_secret(settings: Settings, token: str) -> None:
    config.load_incluster_config()
    encoded_token = base64.b64encode(token.encode("utf-8")).decode("ascii")
    core_api = client.CoreV1Api()
    existing_secret = core_api.read_namespaced_secret(
        name=settings.paperless_mcp_token_secret,
        namespace=settings.pod_namespace,
    )
    existing_data = existing_secret.data or {}
    if existing_data.get("apiKey") == encoded_token:
        log("Paperless MCP token secret already configured")
        return

    core_api.patch_namespaced_secret(
        name=settings.paperless_mcp_token_secret,
        namespace=settings.pod_namespace,
        body={"data": {"apiKey": encoded_token}},
    )
    log("Updated Paperless MCP token secret")


if __name__ == "__main__":
    main()
