import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:a3c44678562ef4e664c7a1dfb75391dc20478fba9355f02e30511c4f7614af8d`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:4a252d68aed4ea6787d185774eeb0593c25d6fd9f067a9117c8253a1299887eb`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:f85d6d9769657935f952e9b3bb7ae6a36c3af4c4beac434ed830346b54ef8f52`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:9bffe84d14c47adc70fa94f416d9048bdaf2caf1c754202b7b24453f219ce2fe`,
};
