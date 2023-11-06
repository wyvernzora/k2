VERSION 0.7
FROM alpine

ansible-base:
    FROM alpine
    RUN adduser -D -h '/ansible' ansible
    RUN apk add --no-cache ansible openssh-client ca-certificates aws-cli py-boto3
    WORKDIR '/ansible'
    USER ansible

proxmox:
    FROM +ansible-base
    COPY "./proxmox" '.'
    RUN ansible-galaxy install -r requirements.yml
    VOLUME ["/ansible/config", "/ansible/.ssh", "/ansible/.aws"]
    CMD [ "ansible-playbook", "/ansible/main.yml" ]
    SAVE IMAGE ghcr.io/wyvernzora/ansible-proxmox:latest
