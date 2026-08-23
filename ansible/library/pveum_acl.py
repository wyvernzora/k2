#!/usr/bin/python

from ansible.module_utils.basic import AnsibleModule
import subprocess
import json

def has_rights(userid, role, path):
    cmd = ["pveum", "acl", "list", "--output-format=json"]
    try:
        output = subprocess.check_output(cmd, universal_newlines=True)
        acl = json.loads(output)
        for e in acl:
            if (e.get('type') == 'user' and
                e.get('ugid') == userid and
                e.get('roleid') == role and
                e.get('path') == path):
                return True
        return False
    except Exception as e:
        raise RuntimeError(f"Error checking ACL: {e}")

def grant_rights(userid, role, path):
    cmd = ["pveum", "acl", "modify", path, "--roles", role, "--users", userid]
    try:
        subprocess.check_call(cmd)
        return True
    except subprocess.CalledProcessError as e:
        return False

def revoke_rights(userid, role, path):
    cmd = ["pveum", "acl", "delete", path, "--roles", role, "--users", userid]
    try:
        subprocess.check_call(cmd)
        return True
    except subprocess.CalledProcessError as e:
        return False

def main():
    module = AnsibleModule(
        argument_spec=dict(
            username=dict(type='str', required=True),
            realm=dict(type='str', default='pam'),
            role=dict(type='str', required=True),
            path=dict(type='str', required=True),
            state=dict(type='str', default='present', choices=['present', 'absent'])
        )
    )

    username = module.params['username']
    realm = module.params['realm']
    role = module.params['role']
    path = module.params['path']
    state = module.params['state']
    userid = f"{username}@{realm}"

    if state == 'present':
        if has_rights(userid, role, path):
            module.exit_json(changed=False, message=f"{userid} already has '{role}' rights on '{path}'.")
        else:
            if grant_rights(userid, role, path):
                module.exit_json(changed=True, message=f"Granted {userid} '{role}' rights on '{path}'.")
            else:
                module.fail_json(msg=f"Failed to grant {userid} '{role}' rights on '{path}'.")
    else:
        if not has_rights(userid, role, path):
            module.exit_json(changed=False, message=f"{userid} does not have '{role}' rights on '{path}'.")
        else:
            if revoke_rights(userid, role, path):
                module.exit_json(changed=True, message=f"Revoked {userid} '{role}' rights on '{path}'.")
            else:
                module.fail_json(msg=f"Failed to revoke {userid} '{role}' rights on '{path}'.")


if __name__ == '__main__':
    main()
