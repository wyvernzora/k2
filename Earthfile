VERSION 0.7
FROM alpine

ansible-base:
    FROM alpine
    RUN adduser -D -h '/ansible' ansible
    RUN apk add --no-cache \
        ansible \
        aws-cli \
        ca-certificates \
        openssh-client \
        py-boto3
    WORKDIR '/ansible'
    USER ansible

proxmox:
    FROM +ansible-base
    COPY "./proxmox" '.'
    RUN ansible-galaxy install -r requirements.yml
    VOLUME ["/ansible/config", "/ansible/.ssh", "/ansible/.aws"]
    CMD [ "ansible-playbook", "/ansible/main.yml" ]
    SAVE IMAGE --push ghcr.io/wyvernzora/k2-ansible-proxmox:latest
