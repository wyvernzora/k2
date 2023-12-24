FROM alpine

# Set up dependencies
RUN apk add --no-cache \
    ansible \
    aws-cli \
    ca-certificates \
    openssh-client \
    py-boto3

# Set up non-root user to run Ansible with
RUN adduser -D -h '/ansible' ansible
WORKDIR '/ansible'
USER ansible

# Copy roles, playbooks etc
COPY ./ ./
RUN ansible-galaxy install -r requirements.yml
ENV ANSIBLE_ROLES_PATH="/ansible/roles:/usr/share/ansible/roles:/etc/ansible/roles"
VOLUME [ "/ansible/.ssh", "/ansible/.aws", "/ansible/group_vars", "/ansible/host_vars", "/ansible/inventory" ]
ENTRYPOINT [ "ansible-playbook" ]
