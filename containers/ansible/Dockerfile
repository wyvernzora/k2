FROM willhallonline/ansible:2.16-alpine-3.18

COPY ansible/requirements.yml .
COPY containers/ansible/entrypoint.sh .

RUN pip install --no-cache-dir botocore boto3 && \
    ansible-galaxy install -r requirements.yml

VOLUME [ "/ansible", "/root/.ssh", "/root/.aws" ]
ENTRYPOINT [ "/ansible/entrypoint.sh" ]
