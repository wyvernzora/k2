# k2-job-runner

`k2-job-runner` is a small utility image for K2 Kubernetes Jobs and init
containers that need common shell, YAML/JSON, DNS, and Kubernetes CLI tools.

It is intended for one-off cluster automation where using a random upstream
utility image would make provenance and tool availability unclear.

## Image

```text
ghcr.io/wyvernzora/k2-job-runner:<tag>
```

CI publishes immutable tags as `sha-<12-char-commit>` on non-PR builds.
Pushes to `main-v3` also publish `latest`.

## Included Tools

- `bash`
- `kubectl`
- `jq`
- `yq`
- `dyff`
- `curl`
- `wget`
- `openssl`
- `envsubst`
- `dig`, `nslookup`
- `nc`
- `tar`, `gzip`, `unzip`
- GNU-style `coreutils`, `findutils`, `grep`, `sed`, `awk`, `diff`

## Build

```sh
earthly +job-runner-image
```

Use `--push` only from an approved publishing workflow.
