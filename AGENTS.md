# AGENTS

## Mission & Tech Stack
- **K2** manages a personal homelab through typed infrastructure-as-code: CDK8s (TypeScript) drives Kubernetes apps, Ansible configures hosts, and Kairos cloud-config templates bootstrap bare metal.
- Runtime stack: Node.js/TypeScript with `tsc`, `tsx`, `cdk8s`, and `cdk8s-plus-32`; Helm charts rendered through the `HelmCharts` context; all secrets are injected from 1Password and TLS from cert-manager.

## Toolchain & Environment
- Install Node 20+, `npm`/`npx`, `tsx`, and `typescript`. Dependencies live in `package.json`; linting/prettier rules sit in `eslint.config.ts` and `package.json.prettier`.
- Earthly (v0.8+) plus Docker/Podman runs every reproducible build defined in `Earthfile`.
- Additional CLIs: `cdk8s`, `helm`, `yq`, `dyff`, `git`, `op` (1Password CLI), and `aws` creds for cert/Ansible roles.
- TypeScript is configured in `tsconfig.json` with path aliases `@k2/cdk-lib` and `@k2/*`, `moduleResolution: nodenext`, strict compiler flags, and ES2024 libs.

## Repository Map
| Path | Purpose |
| --- | --- |
| `apps/<name>/` | Each application module with `Chart.yaml` dependencies, `index.ts` factories, CRD bindings, components, and optional `lib/` helpers. |
| `build/scripts/` | Utility scripts (manifest synth, CRD import, diffing) that the Earthly targets invoke. |
| `cdk-lib/` | Shared CDK8s contexts (`AppRoot`, `HelmCharts`, `Namespace`, `ApexDomain`), constructs (config maps, scheduling, volumes), and helper types for apps. |
| `deploy/` | Output manifests: `deploy/<app>/app.k8s.yaml` (+ CRDs) plus `deploy/app.k8s.yaml` for aggregate ArgoCD applications. |
| `ansible/` | Containerized Ansible runtime (`Earthfile`, `entrypoint.sh`, roles, playbooks) for host bootstrap and TLS refresh. |
| `kairos/` | Kairos cloud-config templates & helper scripts; secrets injected via `op inject`. |
| `Earthfile` | Defines reusable Earthly targets for builds, linting, manifest synthesis, CRD imports, and Docker image publishing. |
| `deploy-diff.md` | Generated report from `build/scripts/diff-manifests.sh`. |
| `package.json`, `package-lock.json`, `node_modules/` | Node dependencies for CDK8s synthesis and linting. |
| `tsconfig.json`, `eslint.config.ts` | Compiler and lint rules that new TypeScript code must follow. |

## Build & Validation Workflows

### Earthly Targets
- `earthly +k8s-manifests` → runs `npx tsx build/scripts/synthesize-manifests.ts`, writing manifests into `deploy/`.
- `earthly +diff-manifests` → compares freshly synthesized manifests against the remote `deploy` branch via `build/scripts/diff-manifests.sh`, honoring `.dyffignore`.
- `earthly +lint` → executes `npx eslint` with the project config (CRD outputs excluded).
- `earthly +crd-constructs` → loops over every `apps/*/crds/crds.k8s.yaml`, runs `build/scripts/generate-crd-constructs.sh`, and stores regenerated TypeScript bindings.
- `earthly +build-image` / `+ansible-image` publish the reusable build/Ansible container images.

### Manual Development Loop
1. `npm ci` (or reuse Earthly `npm-install`) to install dependencies.
2. `npx eslint` to lint TypeScript sources locally.
3. `npx tsx build/scripts/synthesize-manifests.ts` to regenerate manifests in place without Earthly (honors `MAX_CONCURRENCY`).
4. Commit `deploy/` differences to the `deploy` branch (ArgoCD watches that branch).

### CRD Workflow
- If an upstream Helm release adds/changes CRDs: run `build/scripts/generate-crd-manifest.sh apps/<name>` to template CRDs into `apps/<name>/crds/crds.k8s.yaml`, then `build/scripts/generate-crd-constructs.sh apps/<name>` to regenerate TypeScript bindings under `apps/<name>/crds/*.ts`. Generated files stay ignored by ESLint.
- Re-run `+k8s-manifests` afterward to ensure manifests pick up the new CRDs.

### Diffing & Promotion
- Use `build/scripts/diff-manifests.sh <repo-url?> [dyff args]` after synthesis to review manifest changes; it clones the remote `deploy` branch into a temp dir, strips `.git`, and prints Markdown diffs, respecting `.dyffignore`.
- Promote changes by opening PRs against both `main` (source) and the generated `deploy` branch as appropriate.

## CDK8s Architecture & Conventions

### App Composition
- `cdk-lib/App` extends `cdk8s.App`, adds `.use()` to apply `AppOption`s or `Context` classes, and provides `synthToFile()` to write YAML per app.
- `build/scripts/synthesize-manifests.ts` dynamically imports each `apps/<name>/index.ts`, creates a new `App`, applies contexts, calls `createAppResources(app)`, and writes `deploy/<name>/app.k8s.yaml`. It also synthesizes a top-level ArgoCD bundle by calling every `createArgoCdResources(chart)`.
- Always export both `createAppResources` (configure Kubernetes resources) and `createArgoCdResources` (register a `ContinuousDeployment` Argo app) from each `index.ts`.

### Context Pipeline
- `AppRoot` exposes the current app path; `HelmCharts.with()` reads the closest `Chart.yaml` to instantiate chart factories for dependencies and strips CRDs before synthesis.
- `Namespace`/`ApexDomain` contexts let components infer namespaces and domain names without repeated wiring.
- `@k2/1password` provides `VaultContext`/`K2Secret` to pull secrets from the shared vault and surface them as `cdk8s-plus` secrets.
- `@k2/argocd` exposes `ArgoCdContext` and `ContinuousDeployment` constructs wired to repo `deploy` branch, namespace `k2-core`, and auto-sync backoff.

### Scheduling & Storage Helpers
- `cdk-lib/constructs/scheduling.ts` exports reusable tolerations, pod spreads, and node affinities for control-plane workloads.
- `cdk-lib/constructs/volume.ts` layers `K2Volume` factories: `ephemeral`, `replicated` (Longhorn-backed), and `bulk` (NFS-backed, zone-aware).
- `@k2/cert-manager` supplies `K2Issuer` and `K2Certificate` to issue/replicate wildcard TLS using Let’s Encrypt DNS01 via Route53; Traefik’s default `TlsStore` points at `K2Certificate.Name`.

### App Layout Guidelines
- Keep Helm dependencies declared in each app’s `Chart.yaml`; `HelmCharts.of(app).asChart("<alias>")` returns chart classes configured with the matching dependency entry.
- Place constructs and objects to be exported under `apps/<name>/lib/`; actual app deployment constructs under `components/`.
- CRD bindings from upstream charts belong in `apps/<name>/crds/` and should be regenerated via the scripts above.
- Export extra constructs (`export * from "./lib/...";`) when they are intended for reuse in other apps.

### Path Aliases & Imports
- Use `@k2/<app>` imports (configured in `tsconfig.json`) to reuse constructs between apps. Keep import ordering compliant with the ESLint `import/order` rule.

## Secrets, Auth & TLS
- 1Password Connect + Operator live in `apps/1password`; the operator runs in `k2-core` with control-plane tolerations. Use `new K2Secret(scope, id, { itemId })` whenever you need Kubernetes secrets.
- Authentication is centralized via Authelia + Glauth (`apps/auth`). Reuse `Auth.MiddlewareAnnotation` or `AuthenticatedIngress` for Traefik ingress resources to enforce SSO.
- `apps/cert-manager` deploys cert-manager, reflector, Route53-based issuer, and the replicated default TLS certificate.
- Traefik (`apps/traefik`) references the default certificate secret and enables CRD providers, dashboard ingress, and Prometheus annotations; keep middleware names consistent with the auth module.

## Application Catalog (per `apps/*/index.ts`)
| App | Namespace | Highlights |
| --- | --- | --- |
| `1password` | `k2-core` | Deploys 1Password Connect + operator with control-plane tolerations for vault-backed secrets. |
| `argocd` | `k2-core` | Renders the Argo CD Helm chart, disables built-in auth in favor of Authelia, and exposes `/deploy` via Traefik TLS. |
| `auth` | `auth` | Composes Authelia (Helm chart) and a bespoke Glauth LDAP deployment with secrets from 1Password. |
| `cert-manager` | `k2-core` | Installs cert-manager, reflector, Route53-based issuer, and the replicated default TLS certificate. |
| `dns` | `dns` | Deploys k8s-gateway + Blocky with custom block lists, static DNS overrides, and fixed service IP. |
| `gen-ai` | `gen-ai` | Hosts AnythingLLM under `ai.wyvernzora.io`, isolating namespace and apex domain. |
| `home-automation` | `home-automation` | Wraps Home Assistant, Mosquitto, Zigbee2MQTT, etc., with replicated volumes and Zigbee coordinator address. |
| `kube-vip` | `k2-network` | Schedules kube-vip only on control-plane nodes with static VIP `10.10.8.2`. |
| `longhorn` | `k2-storage` | Installs Longhorn and secures its UI with the Authelia middleware. |
| `media` | (per component) | Manages qBittorrent, Prowlarr, Sonarr, and Kavita, wiring replicated PVCs plus NAS-backed bulk storage. |
| `metallb` | `k2-network` | Sets default (`10.10.12.0/24`) and sandbox (`10.10.10.0/24`) address pools with L2 advertisements. |
| `n8n` | `n8n` | Deploys the n8n automation stack with replicated storage and public URL. |
| `plex` | `plex` | Configures Plex with large replicated config storage and read-only media mounts from NAS bulk volumes. |
| `postgresql` | `postgresql` | Installs CloudNativePG operator plus opinionated Nexus cluster resources. |
| `tailscale` | `tailscale` | Manages the Tailscale operator and connector components in a dedicated namespace. |
| `traefik` | `k2-network` | Deploys Traefik with dual-stack services, CRD + ingress providers, dashboard ingress, and TLS store tied to `default-certificate`. |

## Supporting IaC & Provisioning

### Ansible
- Run `docker run --rm -v $PWD/ansible:/ansible ... ghcr.io/wyvernzora/k2-ansible <playbook>`; the entrypoint enumerates playbooks and expects inventory under `/ansible/inventory`, SSH keys under `/root/.ssh`, and AWS credentials for TLS sync.
- Roles (`k2.fish`, `k2.tls`, `k2.user`, `k2.vfio`, `pve.nosub`) are described in `README.md` and are consumed by the `pve-bootstrap` and `update-certs` playbooks.
- The Ansible `Earthfile` builds a container image off `willhallonline/ansible`, installs Galaxy requirements, and sets `/ansible/entrypoint.sh` as the entrypoint.

### Kairos
- Templates in `kairos/*` provide bootstrap/master/worker cloud-configs. Use `op inject -i <file> | pbcopy` to render secrets before pasting into the Kairos installer (see `kairos/README.md`).

## Tips & Gotchas
- Context order matters: `AppRoot` must be registered before `HelmCharts.with()`, and the default synth pipeline already does this—mirror it in tests and scripts.
- Generated CRD bindings under `apps/*/crds/*.ts` should not be edited by hand; regenerate with the scripts and keep them out of lint scope.
- When adding a new app, always create both `createAppResources` and `createArgoCdResources`, list Helm dependencies in `Chart.yaml`, and export any shared constructs from `lib/`.
- Secrets should come from `K2Secret`; direct `Secret` objects break the 1Password workflow.
- Use `ContinuousDeployment` for Argo apps so they inherit the default repo/branch/project, unless you intentionally override `namespace` or `path`.
- After synthesizing manifests, run the diff script before opening PRs to catch unexpected drift.
