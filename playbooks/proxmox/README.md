# Proxmox
This playbook contains my personal Proxmox VE bootstrapping workflow.

## Usage
```shell
$ docker run --rm -it \
    --name ansible-proxmox \
    -e AWS_ACCESS_KEY_ID='<AWS Access Key ID>' \
    -e AWS_SECRET_ACCESS_KEY='<AWS Secret Access Key>' \
    -e AWS_REGION='us-west-2' \
    -v $(pwd):/ansible/config \
    -v $HOME/.ssh:/ansible/.ssh:ro \
    -v $HOME/.aws:/ansible/.aws:ro \
    ghcr.io/wyvernzora/ansible-proxmox:latest
```

### AWS Credentials
The following environment variables are required for fetching TLS certificates from S3.

| Environment Variable    | Description                        |
| ----------------------- | ---------------------------------- |
| `AWS_ACCESS_KEY_ID`     | AWS access key ID                  |
| `AWS_SECRET_ACCESS_KEY` | AWS secret access key              |
| `AWS_SESSION_TOKEN`     | AWS session token if using STS     |
| `AWS_PROFILE`           | AWS profile name if using profiles |
| `AWS_REGION`            | AWS region                         |

Alternatively, you can also mount your `~/.aws` directory to `/ansible/.aws` if you are using
the AWS credentials file. 

### Inventory
Container expects `/ansible/config/inventory.ini` to be mounted and contain inventory info.

Example:
```ini
192.168.1.1
```

### Configuration
Container expects `/ansible/config/vars.yml` to be mounted and contain configuration.

Example:
```yaml
user:
    username: nobody
    password: $2y$10$1MB2S3Bm5pdWZ/pq1lESO.hHbv35WjSZEvjGEZDSa5iAl4LUJ/yo2 # 'password'
    github: nobody

tls:
    domain: example.com
    bucket: s3-bucket-name
```
