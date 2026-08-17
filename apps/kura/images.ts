import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:f55e0f8552f304728fedb5ece0a38ad09a2ce17e46371ffe1246e95e98dd5683`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:5a0448078a37a88a76d82db2d00f89006e295788992cf724838735b2c1974601`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:cfec2cbf862ff56d70628c7a639e8772111fe0fe38495745b4ea5bfd1261574c`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:9e1fa79e214e12006c7a9cca4ac921f8f7ffff9c121c832a076334dbd32c9ac0`,
};
