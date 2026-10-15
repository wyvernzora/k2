import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:6ba7903a4caeb35e912bc72008236f696718f2286c7c83871988a7c7560e007b`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:b16beab3195feb117ed2984b33ab930a57df2d599d9da7f89f215c738a7da52b`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:0f7bdfedbcd3729daff67175083d3ba80baef2e2cef39ea9019e1b2f58b619f5`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:49b5c62787b528c913c513117dfd0c8e3eccbedce5db670cbe324bb816df88aa`,
};
