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
- [`tools/`](tools) – Go toolbox backing build, synth, diff, Kairos, and image workflows.
- [`build/cdk/`](build/cdk) – tiny TypeScript CDK synth entrypoints called by the Go toolbox.
- [`deploy/`](deploy) – synthesized manifests and CRDs checked into the `deploy` branch for ArgoCD.
- [`ansible/`](ansible) – containerized playbooks, roles, and entrypoint for host automation.

## Kubernetes / CDK8s Workflow

1. `npm install` to sync editor-time dependencies.
2. `earthly +k8s-manifests` to regenerate ignored `deploy/`.
3. `earthly +diff-manifests` to review drift against `deploy`.
4. Commit source changes to `main`; promote generated output through the deploy branch workflow.

Manifest synthesis starts from a clean generated `deploy/` tree. Earthly remains the public build interface; the implementation is the Go toolbox under `tools/` plus the minimal CDK-only TypeScript entrypoints in `build/cdk/`.

## Repository Layout

| Path | Description |
| --- | --- |
| `apps/<name>/` | App factory (`createAppResources`/`createArgoCdResources`), Helm dependencies, CRDs, and components. |
| `cdk-lib/` | Shared contexts (namespace, apex domain, Helm), scheduling helpers, and storage constructs. |
| `build/` | Build image definition and CDK-only synth entrypoints. |
| `tools/` | Go toolbox module for build workflows, Kairos operations, image tooling, and shared TUI/workflow code. |
| `deploy/` | Ignored generated manifests; committed through the `deploy` branch workflow. |
| `kairos/` | Kairos image targets, Dockerfile, overlays, node-agent, and provisioning docs. |
