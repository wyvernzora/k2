<div align="center">
    <br>
    <br>
    <img width="182" src="../../../.github/assets/k2.png">
    <h1 align="center">k2.user</h1>
</div>

<p align="center">
<b>Set up non-root user and permissions.</b>
</p>

<hr>
<br>
<br>

## What it does
 - Creates a `wheel` group if one does not already exist
 - Creates a new user along with resource group
 - Sets up SSH keys for the non-root user
 - Adds non-root user to `wheel` group and gives sudo permissions

## Vars and Defaults
| Var Name           | Default Value       | Description                                                        |
| ------------------ | ------------------- | ------------------------------------------------------------------ |
| `k2_user_uid`      | `3000`              | User ID of the non-root user                                       |
| `k2_user_gid`      | `3000`              | Group ID of the non-root user's resource group                     |
| `k2_user_username` | `wyvernzora`        | Username for the non-root user                                     |
| `k2_user_password` | `!`                 | Password for the non-root user. By default disables password login |
| `k2_user_sshkeys`  | `github:wyvernzora` | Authorized SSH keys for the non-root user                          |
| `k2_user_pve_role` | `Administrator`     | ProxmoxVE role to give the non-root user                           |
| `k2_user_pve_path` | `/`                 | ProxmoxVE path for the the user permission                         |
