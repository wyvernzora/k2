import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:9e2c27c957f4028600724f0f99eccf744b1a4c936a5dec22549efd3976230e7d`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:8a30d9259addf1e29a711f036f78ccfaf82d440dce5dcfcdcd74f3238a416216`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:4952a9ccce879b78adf89bc8f35891b68f2d29f50139d5fbdd9ec6792a7f5a7e`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:df3816f412fb7e896711a03f0375865bb15faa1eb8bcb8f92897e551d6c9a2c8`,
};
