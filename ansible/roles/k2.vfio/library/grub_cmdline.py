#!/usr/bin/python

from ansible.module_utils.basic import AnsibleModule
import re

def main():
    module = AnsibleModule(
        argument_spec=dict(
            present=dict(type='list', default=[]),
            absent=dict(type='list', default=[]),
            grub_config=dict(type='str', default='/etc/default/grub')
        )
    )

    present_params = module.params['present']
    absent_params = module.params['absent']
    grub_config_path = module.params['grub_config']

    # Read the GRUB config
    with open(grub_config_path, 'r') as f:
        grub_config = f.read()

    # Extract GRUB_CMDLINE_LINUX_DEFAULT value
    match = re.search(r'^GRUB_CMDLINE_LINUX_DEFAULT="(.*)"$', grub_config, re.MULTILINE)
    if not match:
        module.fail_json(msg="Failed to find GRUB_CMDLINE_LINUX_DEFAULT in {}".format(grub_config_path))
    
    cmdline = match.group(1).split()

    changed = False

    # Ensure parameters are present
    for param in present_params:
        if param not in cmdline:
            cmdline.append(param)
            changed = True

    # Ensure parameters are absent
    for param in absent_params:
        if param in cmdline:
            cmdline.remove(param)
            changed = True

    # If changes were made, update the GRUB config and run update-grub
    if changed:
        new_config = re.sub(r'^(GRUB_CMDLINE_LINUX_DEFAULT=)".*"$', r'\1"{}"'.format(' '.join(cmdline)), grub_config, flags=re.MULTILINE)
        with open(grub_config_path, 'w') as f:
            f.write(new_config)
        rc, stdout, stderr = module.run_command("update-grub")
        if rc != 0:
            module.fail_json(msg=f"Failed to run update-grub: {stderr}")

    module.exit_json(changed=changed)

if __name__ == '__main__':
    main()
