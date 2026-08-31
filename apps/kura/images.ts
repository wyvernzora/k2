import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:fad2dcd1ae09e0a1fb197165dd4f8359eacd2aa570ed68616f0a0daf918df431`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:257b8aac563aeb4f3632b7d40ca0d66da3d4fc48364d6ed12328f0312b89a04d`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:6ae2bd5961701686c16f2431eec2606e1b2fc9ea32e00c09eee06f5d7182109d`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:7b3635bd3994ce52b6e6d11f7b3b839245e4b3e4a9295b39495f5d79671281e7`,
};
