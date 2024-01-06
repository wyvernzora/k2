<div align="center">
    <br>
    <br>
    <img width="182" src="../../../.github/assets/k2.png">
    <h1 align="center">k2.tls</h1>
</div>

<p align="center">
<b>Pull the latest TLS certificates and update them where needed.</b>
</p>

<hr>
<br>
<br>

## What it does
 - Pulls the latest TLS certificates from S3 (see [wyvernzora/cert-bot-lambda](https://github.com/wyvernzora/cert-bot-lambda))
 - Uploads them to remove hosts
 - Performs any follow up action based on the host type
    - Proxmox
        - Restarts nginx
    - TrueNAS
        - Imports the certificate into TrueNAS config store
        - Activates the certificate as the active WebUI cert
        - Removes all other certificates for the same domain
    - (TODO) Unifi
        - Restarts `unifi-core`

## AuthN & AuthZ
Using this role requires proper AWS credentials to read the credential files from the specified S3 bucket.

## Vars and Defaults
| Var Name        | Default Value         | Description                                         |
| --------------- | --------------------- | --------------------------------------------------- |
| `k2_tls_domain` | `wyvernzora.io`       | Domain name for the TLS certificate to pull from S3 |
| `k2_tls_bucket` | `io.wyvernzora.certs` | S3 bucket to grab the latest certificates from      |
