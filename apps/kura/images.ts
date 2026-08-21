import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:1b24e603877030ac97eaaa03e66746e71f561b47e75db7b0da786471772eaddb`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:22a3324ddbd97e3da7a8dafd14de0f5f46a8fdd101d8f7b2a865c2da33bdb5b7`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:d8844c5a53fc5821c389ac4e4817375bcfbede831274e434f8ad22fb5dfc5f78`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:00ccb642f7d845840100b91fc5b766567255c47627d00bec2a2c55d02b638c7b`,
};
