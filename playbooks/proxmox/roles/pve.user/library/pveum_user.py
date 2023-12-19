#!/usr/bin/python
from ansible.module_utils.basic import AnsibleModule
import subprocess
import json

def user_exists(userid):
    cmd = ["pveum", "user", "list", "--output-format=json"]
    try:
        output = subprocess.check_output(cmd, universal_newlines=True)
        users = json.loads(output)
        return any(user.get('userid') == userid for user in users)
    except subprocess.CalledProcessError as e:
        pass
    return False

def add_user(userid):
    cmd = ["pveum", "user", "add", userid]
    try:
        subprocess.check_call(cmd)
        return True
    except subprocess.CalledProcessError as e:
        return False

def delete_user(userid):
    cmd = ["pveum", "user", "delete", userid]
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
            state=dict(type='str', default='present', choices=['present', 'absent'])
        )
    )

    username = module.params['username']
    realm = module.params['realm']
    state = module.params['state']
    userid = f"{username}@{realm}"

    if state == 'present':
        if user_exists(userid):
            module.exit_json(changed=False, message=f"{userid} already exists.")
        else:
            if add_user(userid):
                module.exit_json(changed=True, message=f"Added '{userid}'.")
            else:
                module.fail_json(msg=f"Failed to add '{userid}'.")
    else:
        if not user_exists(userid):
            module.exit_json(changed=False, message=f"{userid} does not exist.")
        else:
            if delete_user(userid):
                module.exit_json(changed=True, message=f"Deleted '{userid}'.")

if __name__ == '__main__':
    main()
