import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:50654042b04b8b2f231fbab8d3ae5c75c5345584b53876a7124b2eec31ada7a4`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:5c3cc1532782cee4047dba434f3716ca976652d36eedbd5173bc78678749d575`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:a6539846b739602a4de7ff441dd419a7d359a8cca579a3e597326b9fbd9afd5a`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:cb1ea4b089a51bc79faecffd19806f7a79c7378ed013ca4e2f7fe1c581a0596e`,
};
