<div align="center">
    <br>
    <br>
    <img width="182" src="../../../.github/assets/k2.png">
    <h1 align="center">k2.tls</h1>
</div>

<p align="center">
<b>Configure host-side TLS plumbing where needed.</b>
</p>

<hr>
<br>
<br>

## What it does
 - Performs any follow up action based on the host type
    - Proxmox
        - Configures nginx to redirect HTTP on port 80 to HTTPS
        - Configures nginx TCP stream passthrough from port 443 to local pveproxy on port 8006

## AuthN & AuthZ
Using this role requires SSH access to the Proxmox hosts.

## Vars and Defaults
This role does not currently expose any variables.
