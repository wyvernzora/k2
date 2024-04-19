FROM willhallonline/ansible:2.15-alpine-3.18

COPY ansible /ansible
COPY containers/ansible/entrypoint.sh /ansible/entrypoint.sh

RUN pip install --no-cache-dir botocore boto3 && \
    ansible-galaxy install -r requirements.yml

VOLUME [ "/ansible", "/root/.ssh", "/root/.aws" ]
ENTRYPOINT [ "/ansible/entrypoint.sh" ]
