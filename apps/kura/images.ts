import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:12b9fc244b5c84159dff15f56661b79595c55ac2958683732262c3100b2a9420`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:b2da424647d893da520528a662a935644ca4884cdcbb60d4b4e3fa32798045e2`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:f27b013ebc2afb548f4cb01e8626a50e2ac63f4716724d2c575c10d75cc138d4`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:472c5994d4e5e10af64828e527251641d41f1bd45e7b833f493237c39d553cfe`,
};
