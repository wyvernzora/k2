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

This image also has a manual contract tag in `image.json`, currently `v2`.
`ScriptedJob` pins to that version tag in `cdk-lib/scripted-job.ts`. Increment
the tag after important runner tool-surface changes, then update `ScriptedJob`
to use the new tag before deploying jobs that need those tools.

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
- Python libraries: `click`, `jsonschema`, `kubernetes`, `requests`, `rich`,
  `websockets`, `yaml`

## Build

```sh
earthly +job-runner-image
```

Use `--push` only from an approved publishing workflow.
