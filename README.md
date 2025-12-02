<div align="center">
    <br>
    <br>
    <img width="256" src=".github/assets/k2.png">
    <h1 align="center">K2</h1>
</div>

<p align="center">
<b>IaC configuration for my homelab.</b>
</p>

<hr>
<br>
<br>

# Overview

K2 is the typed IaC backbone for my homelab: Kubernetes manifests are generated with CDK8s/TypeScript, Ansible keeps the metal hosts consistent, and Kairos templates bootstrap new nodes. Start with [`AGENTS.md`](AGENTS.md) for the full agent handbook, daily workflows, and app catalog.

## Quick Links

- [`AGENTS.md`](AGENTS.md) – canonical guide for agents and contributors.
- [`Earthfile`](Earthfile) – reproducible build/lint targets (manifests, CRDs, Docker images).
- [`build/scripts/`](build/scripts) – automation for manifest synthesis, CRD imports, and diffing.
- [`deploy/`](deploy) – synthesized manifests and CRDs checked into the `deploy` branch for ArgoCD.
- [`ansible/`](ansible) – containerized playbooks, roles, and entrypoint for host automation.

## Kubernetes / CDK8s Workflow

1. `npm install` to sync dependencies.
2. `earthly +k8s-manifests` to regenerate `deploy/`.
3. `earthly +diff-manifests` to review drift against the `deploy` branch.
4. Commit application code to `main`; GitHub actions workflow takes care of the rest.

## Repository Layout

| Path | Description |
| --- | --- |
| `apps/<name>/` | App factory (`createAppResources`/`createArgoCdResources`), Helm dependencies, CRDs, and components. |
| `cdk-lib/` | Shared contexts (namespace, apex domain, Helm), scheduling helpers, and storage constructs. |
| `build/` | Earthly support scripts plus CRD/manifest tooling. |
| `deploy/` | Generated manifests per app plus the aggregated `deploy/app.k8s.yaml`. |
| `ansible/` | Container image definition, roles, and playbooks for Proxmox/bootstrap/TLS tasks. |
| `kairos/` | Cloud-config templates and helper scripts for spinning up Kairos nodes. |
