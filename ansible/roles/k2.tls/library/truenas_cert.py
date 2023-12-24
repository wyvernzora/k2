#!/usr/bin/python
from ansible.module_utils.basic import AnsibleModule
from typing import List, Dict
from types import SimpleNamespace
from datetime import datetime, timezone
import json
import sys
import requests
import subprocess


CERT_API_ENDPOINT = "http://127.0.0.1/api/v2.0/certificate"
SYS_GENERAL_API_ENDPOINT = "http://127.0.0.1/api/v2.0/system/general"
ARGS = dict(
    domain=dict(type='str', required=True),
    state=dict(type='str', default='latest', choices=['latest', "valid"]),
    activate=dict(type='bool', default=True),
    cleanup=dict(type='bool', default=False),
)

class TrueNasCertModule:
    def __init__(self):
        self.module = AnsibleModule(argument_spec=ARGS)
        self.domain = self.module.params["domain"]
        self.state = self.module.params["state"]
        self.activate = self.module.params["activate"]
        self.cleanup = self.module.params["cleanup"]
        self.certfile = f"/etc/ssl/certs/{self.domain}.pem"
        self.keyfile = f"/etc/ssl/private/{self.domain}.pem"
        self.fingerprint = self.get_fingerprint()
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": f"Token {self.generate_token()}"
        }

    #
    # Runs the truenas_cert module
    #
    def run(self):
        if self.cleanup and not self.activate:
            self.module.fail_json(
                message="cleanup can only be true if activation is set to true"
            )
            return

        if self.state == "valid":
            self.cert_valid()
        elif self.state == "latest":
            self.cert_latest()

    #
    # Ensures that a valid certificate is present for the requested domain.
    # Returns the information about the first valid certificate encountered.
    # If no valid certificate is found, uses uploader certificate files to create one.
    #
    def cert_valid(self):
        valid_certs = self.list_certificates(lambda c: cert_is_valid(c) and c.common == self.domain)
        if len(valid_certs) > 0:
            self.cert_cleanup(valid_certs[0])
            self.exit_cert(False, valid_certs[0])
                
        # No valid certs found, so we need to create one
        cert = self.create_certificate()
        self.exit_cert(True, cert)


    #
    # Makes sure that the latest certificate is present in TrueNAS imported certificates.
    # Returns the information about the latest certificate, even if there is already another valid
    # certificate for the requested domain.
    #
    def cert_latest(self):
        cert = self.find_cert_same_fingerprint()
        if cert is None:
            cert = self.create_certificate()
            self.cert_cleanup(cert)
            self.exit_cert(True, cert)
        else:
            self.cert_cleanup(cert)
            self.exit_cert(False, cert)


    #
    # Performs cleanup by activating the chosen certificate and removing all certificates for the requested domain except the latest.
    #
    def cert_cleanup(self, activecert):
        if self.activate:
            self.activate_certificate(activecert)
        if self.cleanup:
            same_domain_certs = self.list_certificates(lambda c: c.common == self.domain and c.fingerprint != activecert.fingerprint)
            for cert in same_domain_certs:
                self.delete_certificate(cert)

    # Computes fingerprint of the desired certificate file
    def get_fingerprint(self) -> str:
        cmd = ["openssl", "x509", "-in", self.certfile, "-noout", "-sha1", "-fingerprint"]
        try:
            output = subprocess.check_output(cmd).decode()

            # Here the format is "sha1 Fingerprint=74:FE:35...", we only need to part after "="
            # Also remove the trailing new line character while we're at it
            return output.split("=")[1].strip()
        except subprocess.CalledProcessError as e:
            raise Exception(e)

    #
    # Uses TrueNAS CLI to generate an auth token that is used for calling the TrueNAS API
    #
    def generate_token(self) -> str:
        cmd = ["/usr/bin/cli", "-c", "auth generate_token"]
        try:
            output = subprocess.check_output(cmd).decode()
            return output[:-1]
        except subprocess.CalledProcessError as e:
            raise Exception(e)

    #
    # Calls TrueNAS API to get the list of all certificates.
    # Optionally applies additional filtering.
    #
    def list_certificates(self, filter = lambda: True):
        certs = self.request('GET', CERT_API_ENDPOINT)
        return [cert for cert in certs if filter(cert)]

    #
    # Looks for a certificate that has the same fingerprint as the latest.
    # Returns None if none are found.
    #
    def find_cert_same_fingerprint(self):
        match = self.list_certificates(lambda cert: self.fingerprint == cert.fingerprint)
        if len(match) > 0:
            return match[0]
        else:
            return None

    #
    # Lists all certificates that share the requested domain.
    # Used for cleaning up old certificates
    #
    def find_cert_same_domain(self):
        return self.list_certificates(lambda cert: cert.common == self.domain)

    #
    # Calls TrueNAS API to create a new certificate using latest files uploaded.
    #
    def create_certificate(self):
        data = {
            "create_type": "CERTIFICATE_CREATE_IMPORTED",
            "certificate": read_file(self.certfile),
            "privatekey": read_file(self.keyfile),
            "name": self.generate_certificate_name(),
        }
        self.request('POST', CERT_API_ENDPOINT, data)
        cert = self.find_cert_same_fingerprint()
        if cert is None:
            raise Exception("Missing cert that was just created!")
        return cert

    #
    # Calls TrueNAS API to delete a certificate.
    #
    def delete_certificate(self, cert):
        self.request('DELETE', f"{CERT_API_ENDPOINT}/id/{cert.id}")

    #
    # Activates a certificate as the one used for web UI
    #
    def activate_certificate(self, cert):
        self.request('PUT', SYS_GENERAL_API_ENDPOINT, {
            "ui_certificate": cert.id,
        })
        self.request('GET', f"{SYS_GENERAL_API_ENDPOINT}/ui_restart")

    #
    # Generates a certificate name unique to the domain and fingerprint.
    #
    def generate_certificate_name(self) -> str:
        return f"{self.domain.replace('.', '-')}-{self.fingerprint.replace(':', '')[-8:]}"

    #
    # Sends an authenticated request to TrueNAS API
    #
    def request(self, method: str, url: str, body = None):
        if body is not None:
            body = json.dumps(body)
        resp = requests.request(method, url, headers=self.headers, data=body, verify=False)
        if resp.status_code != 200:
            raise Exception(resp.status_code, resp.content)
        return resp.json(object_hook=lambda d: SimpleNamespace(**d))

    #
    # Passes on information about certificate back to Ansible
    #
    def exit_cert(self, changed: bool, cert):
        self.module.exit_json(
            changed=changed,
            name=cert.name, 
            id=cert.id,
            fingerprint=cert.fingerprint,
            valid_from=getattr(cert, 'from'),
            valid_until=cert.until,
        )


def cert_is_valid(cert) -> bool:
    date_format = "%a %b %d %H:%M:%S %Y"
    valid_from = datetime.strptime(getattr(cert, 'from'), date_format)
    valid_until = datetime.strptime(cert.until, date_format)

    now = datetime.now()
    return valid_from <= now <= valid_until


def read_file(path) -> str:
    with open(path, 'r') as file:
        return file.read()
    
def main():
    TrueNasCertModule().run()

if __name__ == '__main__':
    main()
