# AGENTS.md

Drop-in operating instructions for coding agents working on **K2**. Read the
user-global rules first:

- `~/.agents/AGENTS.md` - universal agent-behavior rules, if present.
- `~/.agents/typescript.md` - TypeScript engineering rules, if present.
- `~/.agents/go.md` - Go engineering rules, if present, for `kairos/image-build`.

This file holds K2-specific context, hard boundaries, and accumulated project
learnings. Global rules apply unless this file explicitly overrides them.

---

## 1. Project-Specific Overrides

### Earthly Is The Build Interface

- Build, lint, manifest synthesis, CRD construct generation, manifest diffing,
  and image workflows must run through `earthly` targets.
- Host-side Node/npm installs are for editor/dev-time type checking only. Do
  not use host `npm`, `npx`, `tsx`, `tsc`, eslint, or direct scripts as the
  source of validation.
- Scripts under `build/scripts/` are target implementation details. Do not
  invoke them directly for normal validation unless the user explicitly asks
  for script-level debugging.
- If Earthly fails because Docker/Podman/network access is unavailable, say so
  plainly and report what was not validated.

### CRD Bindings Are Mandatory

- Never hand-write raw `ApiObject` for a Kubernetes custom resource when a CRD
  is available.
- Put CRD manifests under the owning app at `apps/<name>/crds/crds.k8s.yaml`.
- Generate TypeScript bindings with `earthly +crd-constructs`.
- CRD-specific helpers live with the owning app and are exported through
  `@k2/<app>`. They do not belong in generic `cdk-lib`.
- Generated CRD bindings may be ignored by Git; regenerate them before lint or
  synth when needed.

### Commit Hygiene

- Keep review-distinct changes in separate commits when the user asks for
  commits or history surgery.
- When amending a stack, preserve user edits and unrelated dirty files. Do not
  reset or revert work you did not make unless the user explicitly requests it.
- Generated `deploy/` output is ignored on the source branch. Commit source
  changes here; promote generated manifests through the deploy branch workflow
  when that is part of the task.

---

## 2. Project Context

### About K2

- **Name:** K2.
- **Domain:** personal homelab infrastructure-as-code.
- **Purpose:** manage Kubernetes applications, cluster bootstrap, and Kairos
  bare-metal images with typed, reviewable source.
- **Current source branch:** `main-v3`, a greenfield v3 branch. Legacy v2 source
  remains on `main` until cutover.
- **Current deploy branch target:** `deploy-v3`.

### Stack

- **Kubernetes IaC:** CDK8s in TypeScript, using `cdk8s`, `cdk8s-plus-32`,
  `tsx`, and strict `tsconfig.json` settings.
- **Helm integration:** app `Chart.yaml` dependencies are loaded through the
  `HelmCharts` context.
- **CRDs:** generated CDK8s TypeScript bindings from app-owned CRD manifests.
- **Build system:** Earthly v0.8+ with Docker or Podman.
- **Kairos image work:** Go CLI under `kairos/image-build`.
- **Secrets and TLS direction:** secrets should come from 1Password-backed app
  helpers; cert-manager concerns should live in `@k2/cert-manager` when that
  app exists, not in generic cluster config.

### Repository Map

- `apps/<name>/` - one Kubernetes app module. The directory name is the app
  name and namespace.
- `apps/<name>/components/` - app deployment constructs.
- `apps/<name>/lib/` - app-owned reusable helpers exported through
  `@k2/<app>`.
- `apps/<name>/crds/` - upstream CRD manifest and generated bindings for that
  app.
- `cdk-lib/` - shared app-agnostic CDK8s primitives, contexts, scheduling,
  workload helpers, and volume helpers.
- `cdk-lib/volumes/` - volume base and one file per concrete volume type.
- `build/scripts/` - Earthly target implementation scripts.
- `clusters/v3.yaml` - the single v3 cluster config file.
- `deploy/` - ignored generated manifests from `earthly +k8s-manifests`.
- `kairos/` - Kairos image targets, versions, Earthly targets, and image build
  tooling.
- `notes/` - ignored design checkpoints and local planning notes.

### Commands

```sh
earthly +crd-constructs      # generate app CRD TypeScript bindings
earthly +lint                # regenerate CRD bindings in-container, typecheck, lint
earthly +k8s-manifests       # synthesize deploy/ from a clean generated tree
earthly +diff-manifests      # compare fresh deploy/ against remote deploy-v3
earthly +build-image         # publish the reusable K2 build image
```

For Kairos image-build development:

```sh
cd kairos/image-build
go test ./...
go run ./cmd/image-build --help
```

Use Go commands directly only for the Go-only Kairos image-build module. Use
Earthly for K2 CDK8s, manifests, lint, and CRD workflows.

---

## 3. CDK8s v3 Architecture

### App Shape

- One `apps/<name>/` directory equals one Kubernetes namespace named `<name>`.
- One app directory synthesizes through one `cdk8s.App`.
- `K2App` uses `YamlOutputType.FILE_PER_APP`, producing
  `deploy/apps/<name>/app.k8s.yaml`.
- App CRDs, if present, are copied to `deploy/apps/<name>/crds.k8s.yaml`.
- Argo CD Application names are exactly the app directory names. Do not add a
  `v3-` prefix.

### App Exports

Every `apps/<name>/index.ts` exports typed named constants:

```ts
export const createAppResources: AppResourceFunc = app => {
  // create component charts/resources
};

export const createArgoCdApp: ArgoCdAppFunc = defaultArgoCdAppFunc({
  // optional app-specific Argo settings
});
```

- Do not export loose untyped functions.
- Do not add `defineDeployment`.
- Do not add `export const deployment`.
- Cluster bootstrap (provisioning, initial control-plane setup, installing
  cilium/kube-vip/argocd before GitOps takes over) is handled by the
  provisioner CLI in `kairos/`, *not* by the cdk8s layer. Synthesized
  manifests do not encode bootstrap ordering and do not emit Argo CD
  sync-wave annotations. Kubernetes converges eventually; the manifest
  layer treats every app identically.

### Contexts

- Pass construct-time facts through `Context.of(this)` inside constructs.
- Do not pass a synthetic context object like `K2SynthContext` into app
  factories.
- Keep `cdk-lib/context/` narrow and app-agnostic: current examples are
  `AppRoot`, `HelmCharts`, `Namespace`, `ApexDomain`, and `NfsContext`.
- Do not add generic `AuthContext`, `CertContext`, or `NetworkContext`.
  Import app-owned helpers from `@k2/auth`, `@k2/cert-manager`,
  `@k2/cilium`, etc.

### Cluster Config Boundary

`clusters/v3.yaml` is only for truly cluster-wide values that matter to
multiple constructs inside Kubernetes context.

Keep out of cluster YAML:

- ingress defaults
- load-balancer pools until there is a concrete app need
- default certificate names
- auth middleware names
- Cilium policy defaults
- application-side configuration
- bootstrap membership

Default cert details belong in `@k2/cert-manager`. Auth details belong in
`@k2/auth`. Cilium CRD resources and network policy helpers belong in
`@k2/cilium`.

### Shared Library Layout

- `cdk-lib/` is already a construct/helper library. Do not create a nested
  `cdk-lib/constructs/` namespace.
- Keep shared code app-agnostic. If a helper depends on an app CRD, move it to
  that app.
- Split broad helper families by type. Volumes live under `cdk-lib/volumes/`
  with separate files for ephemeral, NFS, replicated, and shared base types.

### Network Policy

- Cilium owns the network policy DSL because it depends on Cilium CRDs.
- Use generated `CiliumNetworkPolicy` bindings from `apps/cilium/crds/`.
- The synth layer does not apply default-deny automatically. Apps that
  want a namespace-wide default-deny opt in by instantiating
  `DefaultDenyNetworkPolicy` from `@k2/cilium` in their own
  `createAppResources`. Apps that need exceptions (e.g. cilium itself,
  hostNetwork pods) simply do not instantiate it, or pair it with
  explicit allow `CiliumNetworkPolicy` rules.
- Cross-cutting presets such as DNS, API server, ingress controller, and
  monitoring access should be typed helpers in `@k2/cilium`.

---

## 4. Workflows

### Normal Validation Loop

1. Run `earthly +crd-constructs` after CRD manifest changes or when ignored
   generated bindings are missing.
2. Run `earthly +lint`.
3. Run `earthly +k8s-manifests`.
4. Run `earthly +diff-manifests` after a fresh synth when the remote
   `deploy-v3` branch exists.
5. Inspect generated manifests when behavior matters; do not rely only on
   source-level reasoning.

### Adding An App

1. Create `apps/<name>/Chart.yaml` for Helm dependencies, if any.
2. Put deployment constructs under `apps/<name>/components/`.
3. Put reusable app-owned helpers under `apps/<name>/lib/` and export them
   from `apps/<name>/index.ts` when other apps should import them.
4. Export typed named `createAppResources` and `createArgoCdApp`.
5. Add app CRDs under `apps/<name>/crds/` when the app owns custom resources.
6. Run the Earthly validation loop.

### Updating CRDs

1. Update `apps/<name>/crds/crds.k8s.yaml`.
2. Run `earthly +crd-constructs`.
3. Use the generated TypeScript binding for custom resources.
4. Run `earthly +lint` and `earthly +k8s-manifests`.

### Kairos Image Work

- Kairos v3 image work is authoritative on `main-v3`.
- Prefer the reproducible Earthly image artifact path for image outputs.
- Direct Go commands are acceptable inside `kairos/image-build` while iterating
  on that Go CLI.

---

## 5. Forbidden

- Raw `ApiObject` for any custom resource with an available CRD.
- Cilium CRD helpers in `cdk-lib`.
- Generic auth/cert/network contexts in `cdk-lib`.
- Bootstrap-aware logic in the manifest synth (sync waves, bootstrap
  policy maps, default-deny opt-out lists, etc.). Bootstrap is the
  provisioner CLI's job, not the cdk8s layer's.
- `v3-` prefixes on Argo CD Application names.
- `FILE_PER_CHART` output for app manifests.
- Nested `cdk-lib/constructs/`.
- App-side configuration in `clusters/v3.yaml`.
- Host-side npm/node commands as build/lint/synth validation.
- Direct edits to generated CRD bindings when regeneration is the right fix.
- Reverting user changes or unrelated dirty files during history surgery.

---

## 6. Project Learnings

**Accumulated corrections. This section is for the agent to maintain, not just
the human.**

When the user corrects your approach, append a one-line rule here before ending
the session. Write it concretely ("Always use X for Y"), never abstractly ("be
careful with Y"). If an existing line already covers the correction, tighten it
instead of adding a new one. Remove lines when the underlying issue goes away.

- Argo CD Application names are the app directory names; never prefix them with
  the cluster name.
- App factories receive the `K2App` only; constructs read cluster/app facts
  with `Context.of(this)`.
- App module shape is typed named exports:
  `createAppResources: AppResourceFunc` and `createArgoCdApp: ArgoCdAppFunc`.
- `createArgoCdApp` should usually be assigned with
  `defaultArgoCdAppFunc({ ... })`.
- Cluster bootstrap is the provisioner CLI's concern, not the cdk8s
  layer's. Do not reintroduce bootstrap maps or sync-wave annotations
  into synth.
- One app directory means one app manifest file; keep `YamlOutputType.FILE_PER_APP`.
- Keep `cdk-lib` flat and app-agnostic; do not recreate a
  `cdk-lib/constructs` subtree.
- Split volume implementations by type under `cdk-lib/volumes`.
- App-local constructs used only by an app's own `components/` stay under that
  app's `components/`; reserve `apps/<name>/lib/` for constructs imported by
  other apps through `@k2/<name>`.
- Do not add `AuthContext`, `CertContext`, or `NetworkContext`; import concrete
  helpers from the owning app packages.
- Network policy helpers live in `@k2/cilium` and use generated Cilium CRD
  bindings.
- Host-side npm dependencies are for dev-time editor/type assistance only; all
  real validation is through Earthly.
- Cluster YAML is for real cluster-wide Kubernetes context only; app-configured
  values stay app-side.
- Keep ingress defaults and load-balancer pools out of cluster YAML until a
  concrete app needs them.
- Default certificate names belong in `@k2/cert-manager`, not cluster config.
- App-facing secret constructs must stay backend-neutral; do not expose whether
  ordinary secrets currently come from 1Password, AWS Secrets Manager, or
  another backend.
- Shared secret provider auth and stores belong to `external-secrets`. Prefer
  WebIdentity for AWS runtime access; do not add a generic AWS credential
  Secret construct until a concrete workload requires literal SigV4 keys.
- For live v3 cluster diagnostics, use the explicit kubeconfig at
  `/Users/wyvernzora/.kube/k2/k2/kubeconfig`; the ambient context may point at
  the legacy API VIP.
- Cert-manager Route53 DNS01 uses K2's public service-account OIDC issuer and
  cert-manager `auth.kubernetes.serviceAccountRef`; do not route DNS01 through
  ESO or `aws-sts-bootstrap`.
- Raw generated CRD constructs exported from an app package must be namespaced
  behind `crd`, e.g. `import { crd } from "@k2/external-secrets"`.
- Bootstrap provisioning must apply the root Argo CD app-of-apps after the
  Argo Application CRD is established; do not rely on K3s static manifest
  ordering for Argo `Application` custom resources.
- The root Argo CD app-of-apps should not auto-sync; generated child
  Applications should own normal auto-sync behavior.
