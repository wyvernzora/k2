#!/usr/bin/python

import os
import re
import shlex
import tempfile

from ansible.module_utils.basic import AnsibleModule


def updated_parameters(current, present, absent):
    parameters = [item for item in current if item not in absent]
    for item in present:
        if item not in parameters:
            parameters.append(item)
    return parameters


def atomic_write(path, content):
    stat = os.stat(path)
    directory = os.path.dirname(path)
    descriptor, temporary_path = tempfile.mkstemp(dir=directory)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as temporary_file:
            temporary_file.write(content)
        os.chmod(temporary_path, stat.st_mode)
        os.chown(temporary_path, stat.st_uid, stat.st_gid)
        os.replace(temporary_path, path)
    finally:
        if os.path.exists(temporary_path):
            os.unlink(temporary_path)


def update_grub(module, present, absent, path):
    try:
        with open(path, "r", encoding="utf-8") as grub_file:
            original = grub_file.read()
    except OSError as error:
        module.fail_json(msg=f"Failed to read {path}: {error}")

    match = re.search(
        r'^GRUB_CMDLINE_LINUX_DEFAULT="(.*)"$', original, re.MULTILINE
    )
    if not match:
        module.fail_json(msg=f"Failed to find GRUB_CMDLINE_LINUX_DEFAULT in {path}")

    current = shlex.split(match.group(1))
    updated = updated_parameters(current, present, absent)
    if updated == current:
        module.exit_json(changed=False, bootloader="grub", parameters=current)

    replacement = f'GRUB_CMDLINE_LINUX_DEFAULT="{shlex.join(updated)}"'
    rendered = re.sub(
        r'^GRUB_CMDLINE_LINUX_DEFAULT=".*"$', replacement, original, flags=re.MULTILINE
    )
    if not module.check_mode:
        atomic_write(path, rendered)
        rc, _, stderr = module.run_command(["update-grub"])
        if rc != 0:
            module.fail_json(msg=f"Failed to run update-grub: {stderr}")

    module.exit_json(changed=True, bootloader="grub", parameters=updated)


def update_proxmox_boot_tool(module, present, absent, path):
    try:
        with open(path, "r", encoding="utf-8") as cmdline_file:
            original = cmdline_file.read()
    except OSError as error:
        module.fail_json(msg=f"Failed to read {path}: {error}")

    current = shlex.split(original.strip())
    updated = updated_parameters(current, present, absent)
    if updated == current:
        module.exit_json(
            changed=False, bootloader="proxmox-boot-tool", parameters=current
        )

    if not module.check_mode:
        atomic_write(path, f"{shlex.join(updated)}\n")
        rc, _, stderr = module.run_command(["proxmox-boot-tool", "refresh"])
        if rc != 0:
            module.fail_json(msg=f"Failed to refresh Proxmox boot entries: {stderr}")

    module.exit_json(
        changed=True, bootloader="proxmox-boot-tool", parameters=updated
    )


def main():
    module = AnsibleModule(
        argument_spec={
            "present": {"type": "list", "elements": "str", "default": []},
            "absent": {"type": "list", "elements": "str", "default": []},
            "bootloader": {
                "type": "str",
                "required": True,
                "choices": ["grub", "proxmox-boot-tool"],
            },
            "grub_config": {"type": "path", "default": "/etc/default/grub"},
            "kernel_cmdline": {"type": "path", "default": "/etc/kernel/cmdline"},
        },
        supports_check_mode=True,
    )

    if module.params["bootloader"] == "grub":
        update_grub(
            module,
            module.params["present"],
            module.params["absent"],
            module.params["grub_config"],
        )
    else:
        update_proxmox_boot_tool(
            module,
            module.params["present"],
            module.params["absent"],
            module.params["kernel_cmdline"],
        )


if __name__ == "__main__":
    main()
