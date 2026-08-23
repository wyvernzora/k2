#!/usr/bin/python

from ansible.module_utils.basic import AnsibleModule
import json


def read_realm(module, realm):
    rc, stdout, stderr = module.run_command(
        ["pvesh", "get", f"/access/domains/{realm}", "--output-format", "json"]
    )
    if rc == 0:
        return json.loads(stdout)
    if "does not exist" in stderr:
        return None
    module.fail_json(msg=f"failed to read PVE realm {realm}: {stderr.strip()}")


def normalized(value):
    if isinstance(value, bool):
        return int(value)
    return value


def main():
    module = AnsibleModule(
        argument_spec={
            "realm": {"type": "str", "required": True},
            "issuer_url": {"type": "str", "required": True},
            "client_id": {"type": "str", "required": True},
            "client_key": {"type": "str", "required": True, "no_log": True},
            "autocreate": {"type": "bool", "default": True},
            "username_claim": {"type": "str", "default": "username"},
            "scopes": {"type": "str", "default": "profile email"},
            "default": {"type": "bool", "default": False},
            "comment": {"type": "str", "default": "K2 Pocket ID"},
        },
        supports_check_mode=True,
    )

    realm = module.params["realm"]
    desired = {
        "type": "openid",
        "issuer-url": module.params["issuer_url"],
        "client-id": module.params["client_id"],
        "client-key": module.params["client_key"],
        "autocreate": int(module.params["autocreate"]),
        "username-claim": module.params["username_claim"],
        "scopes": module.params["scopes"],
        "default": int(module.params["default"]),
        "comment": module.params["comment"],
    }
    current = read_realm(module, realm)

    if current is not None and current.get("type") != "openid":
        module.fail_json(msg=f"PVE realm {realm} exists with type {current.get('type')}, expected openid")

    if current is not None and current.get("username-claim") != desired["username-claim"]:
        module.fail_json(
            msg=(
                f"PVE realm {realm} has immutable username-claim "
                f"{current.get('username-claim')!r}, expected {desired['username-claim']!r}"
            )
        )

    changed_keys = [
        key
        for key, value in desired.items()
        if current is None
        or normalized(current.get(key, 0 if key == "default" else None)) != normalized(value)
    ]
    changed = bool(changed_keys)
    if not changed or module.check_mode:
        module.exit_json(changed=changed)

    command = ["pveum", "realm", "add" if current is None else "modify", realm]
    if current is None:
        command.extend(["--type", "openid"])
    for key in changed_keys:
        if key == "type":
            continue
        value = desired[key]
        command.extend([f"--{key}", str(value)])

    rc, _, stderr = module.run_command(command)
    if rc != 0:
        module.fail_json(msg=f"failed to configure PVE realm {realm}: {stderr.strip()}")
    module.exit_json(changed=True)


if __name__ == "__main__":
    main()
