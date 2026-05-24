# AGENTS.md

Drop-in operating instructions for coding agents working on **K2**. Read the
user-global rules first:

- `~/.agents/AGENTS.md` - universal agent-behavior rules, if present.
- `~/.agents/typescript.md` - TypeScript engineering rules, if present.
- `~/.agents/go.md` - Go engineering rules, if present, for
  `kairos/image-build`.

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
- Use generated TypeScript bindings for custom resources.
- CRD-specific helpers live with the owning app. They do not belong in generic
  `cdk-lib`.
- Raw generated CRD constructs exported from an app package must be namespaced
  behind `crd`, e.g. `import { crd } from "@k2/external-secrets"`.
- Generated CRD bindings may be ignored by Git; regenerate them before lint or
  synth when needed. Do not edit generated bindings directly when regeneration
  is the right fix.

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
- **Current source branch:** `main-v3`, a greenfield v3 branch. Legacy v2
  source remains on `main` until cutover.
- **Current deploy branch target:** `deploy-v3`.

### Stack

- **Kubernetes IaC:** CDK8s in TypeScript, using `cdk8s`, `cdk8s-plus-32`,
  `tsx`, and strict `tsconfig.json` settings.
- **Helm integration:** app `Chart.yaml` dependencies are loaded through the
  `HelmCharts` context.
- **CRDs:** generated CDK8s TypeScript bindings from app-owned CRD manifests.
- **Build system:** Earthly v0.8+ with Docker or Podman.
- **Kairos image work:** Go CLI under `kairos/image-build`.
- **Secrets and TLS:** backend-neutral secret helpers live in
  `@k2/external-secrets`; certificate defaults and replication live in the
  cert-manager app; AWS runtime access should prefer WebIdentity.

### Repository Map

- `apps/<name>/` - one Kubernetes app module. The directory name is the app
  name and namespace.
- `apps/<name>/components/` - deployable app components. Each direct item is a
  logical deployable unit, roughly a Kubernetes `Chart`.
- `apps/<name>/lib/` - app-owned reusable helpers exported through
  `@k2/<app>` for other apps to import.
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
- `notes/` and `.checkpoint/` - ignored design checkpoints and local planning
  notes.

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

### App Model

- One `apps/<name>/` directory equals one Kubernetes namespace named `<name>`.
- One app directory synthesizes through one `cdk8s.App`.
- `K2App` uses `YamlOutputType.FILE_PER_APP`, producing
  `deploy/<name>/app.k8s.yaml`. The root app-of-apps bundle synthesizes to
  `deploy/app.k8s.yaml`.
- App CRDs, if present, are copied to `deploy/<name>/crds.k8s.yaml`.
- Argo CD Application names are exactly the app directory names. Do not add a
  `v3-` prefix.
- Every `apps/<name>/index.ts` exports one typed named constant:

```ts
export const createAppResources: AppResourceFunc = app => {
  // create component charts/resources
};
```

- Do not export loose untyped functions.
- Do not export `createArgoCdApp`; synth derives Argo CD Applications
  uniformly from the app directory and cluster config.
- Do not add `defineDeployment`.
- Do not add `export const deployment`.

### Component Layout

- Every direct item under `apps/<name>/components/` is a logical deployable
  unit wired by `createAppResources`.
- Prefer `apps/<name>/components/<component>/index.ts` plus neighboring
  construct files for components with multiple resources or more than about 100
  SLOC.
- A simple component may stay as a single `components/<component>.ts` file to
  avoid pointless nesting.
- App-local constructs used only by an app's own component stay under that
  app's `components/`; reserve `apps/<name>/lib/` for constructs intentionally
  imported by other apps through `@k2/<name>`.
- Wire only the component facade from the app module. Component internals
  should be imported by neighboring files inside the component subtree.

### Resource Construction Style

- Top-level component constructors should read as orchestration, not a wall of
  manifest shape.
- When a constructor both orchestrates several things and instantiates a
  resource with a large props object, especially a raw CRD, wrap that resource
  in a named construct extending the resource type and put it in a dedicated
  component-local file.
- Alias excessively long generated CRD enum/type names near the top of the
  dedicated resource file so the props body remains readable.
- Object literals nested more than about three levels deep should usually move
  into named helper functions in the same file, unless the whole file is already
  a dedicated resource wrapper.

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

### Secrets, TLS, And AWS

- App-facing secret constructs must stay backend-neutral. Do not expose whether
  ordinary secrets currently come from 1Password, AWS Secrets Manager, or
  another backend.
- Shared secret provider auth and stores belong to `external-secrets`, and all
  secret-related provider infrastructure should live in the `external-secrets`
  namespace.
- Final app-consumed Kubernetes Secrets may still exist in app namespaces
  because Kubernetes Secret consumption is namespace-scoped.
- Prefer WebIdentity for AWS runtime access. Do not add a generic AWS
  credential Secret construct until a concrete workload requires literal SigV4
  keys.
- Cert-manager Route53 DNS01 uses K2's public service-account OIDC issuer and
  cert-manager `auth.kubernetes.serviceAccountRef`; do not route DNS01 through
  ESO or `aws-sts-bootstrap`.
- Default certificate names and TLS replication behavior belong in the
  cert-manager app, not cluster config.

### Network Policy

- Cilium owns the network policy DSL because it depends on Cilium CRDs.
- Use generated `CiliumNetworkPolicy` bindings from `apps/cilium/crds/`.
- Treat namespaces as the default trust boundary. Apps opt into enforcement by
  instantiating `NamespaceBoundaryPolicy` from `@k2/cilium`.
- A namespace boundary allows same-namespace traffic and kube-apiserver access;
  cross-namespace and outside-cluster relationships need explicit allow
  policies.
- Caller/callee relationships are normally owned by the caller. For
  one-to-many relationships, each spoke owns its own edge. True peer
  relationships are `REVISIT` until there is a real case.
- The app that owns a K2 pod owns policies for that pod's traffic to or from
  outside-cluster peers.
- Cross-cutting presets such as DNS, API server, ingress controller, and
  monitoring access should be typed helpers in `@k2/cilium`.

### Scheduling

- Movable workloads should prefer worker nodes.
- Movable-but-critical data-plane workloads should prefer workers and tolerate
  control-plane nodes only as fallback.
- Host/control-plane style workloads, such as `kube-vip`, may remain
  control-plane pinned.

### Bootstrap And GitOps

- Cluster bootstrap is the provisioner CLI's concern, not the cdk8s layer's.
- Synthesized manifests do not encode bootstrap ordering and do not emit Argo
  CD sync-wave annotations. Kubernetes converges eventually; the manifest layer
  treats every app identically.
- Bootstrap provisioning must apply the root Argo CD app-of-apps after the Argo
  Application CRD is established; do not rely on K3s static manifest ordering
  for Argo `Application` custom resources.
- The root Argo CD app-of-apps should not auto-sync; generated child
  Applications should own normal auto-sync behavior.

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
2. Put deployment components under `apps/<name>/components/`; use a component
   subdirectory with `index.ts` when the component needs multiple construct
   files.
3. Put reusable app-owned helpers under `apps/<name>/lib/` and export them
   from `apps/<name>/index.ts` when other apps should import them.
4. Export typed named `createAppResources: AppResourceFunc`.
5. Add app CRDs under `apps/<name>/crds/` when the app owns custom resources.
6. Run the Earthly validation loop.

### Updating CRDs

1. Update `apps/<name>/crds/crds.k8s.yaml`.
2. Run `earthly +crd-constructs`.
3. Use the generated TypeScript binding for custom resources.
4. Run `earthly +lint` and `earthly +k8s-manifests`.

### Live Cluster Diagnostics

- For live v3 cluster diagnostics, use the explicit kubeconfig at
  `/Users/wyvernzora/.kube/k2/k2/kubeconfig`; the ambient context may point at
  the legacy API VIP.
- Do not reveal Secret values in command output or summaries.

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
- Bootstrap-aware logic in the manifest synth, including sync waves, bootstrap
  policy maps, and default-deny opt-out lists.
- `v3-` prefixes on Argo CD Application names.
- `FILE_PER_CHART` output for app manifests.
- Nested `cdk-lib/constructs/`.
- App-side configuration in `clusters/v3.yaml`.
- Host-side npm/node commands as build/lint/synth validation.
- Direct edits to generated CRD bindings when regeneration is the right fix.
- Reverting user changes or unrelated dirty files during history surgery.

---

## 6. Project Learnings Inbox

This section is intentionally short. When the user corrects your approach,
either tighten the stable rule above or append one concrete one-line rule here
before ending the session. During grooming, promote durable rules into the
proper section above and remove the inbox duplicate.

No ungroomed learnings currently.
