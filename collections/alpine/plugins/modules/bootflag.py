#!/usr/bin/python

import os
import re
import tempfile

# import module snippets
from ansible.module_utils.basic import AnsibleModule

BOOTFLAGS_PATTERN = re.compile(r'default_kernel_opts="([^"]+)"')


def parse_bootflags(input):
    match = BOOTFLAGS_PATTERN.search(input)
    return match.group(1).split(" ")

def main():
    module = AnsibleModule(
        argument_spec=dict(
            flag=dict(type='list', elements='str', aliases=['flags']),
            state=dict(type='str', default='present', choices=['absent', 'present']),
        ),
    )

    params = module.params
    flags = params['flag']
    state = params['state']
    
    # Read the extlinux config file into memory
    lines = []
    with open('/etc/update-extlinux.conf') as file:
        for line in file:
            lines.append(line)
    
    # Find and parse bootflags
    bootflags = next(filter(lambda line: BOOTFLAGS_PATTERN.match(line), lines))
    bootflags = parse_bootflags(bootflags)

    changed = False
    for flag in flags:
        if state == 'present' and not (flag in bootflags):
            bootflags.append(flag)
            changed = True
        if state == 'absent' and (flag in bootflags):
            bootflags.remove(flag)
            changed = True

    # Write results
    if changed:
        with open('/etc/update-extlinux.conf', 'w') as file:
            for line in lines:
                if not BOOTFLAGS_PATTERN.match(line):
                    file.write(line)
                else:
                    file.write('default_kernel_opts="' + ' '.join(bootflags) + '"')

    module.exit_json(changed=changed, bootflags=bootflags)

if __name__ == "__main__":
  main()
