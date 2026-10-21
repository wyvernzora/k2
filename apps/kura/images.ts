import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:edaa4d0d19159b575248dec3cdb0723da01e56df1cd079c35827082d42fd36f2`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:e7992995fd75670454bd2e2b0bc51116bb84743684d16732b0011e73066b04e0`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:821c77476cbed607ba196f518d5068e2c62b270fa8a0853fcde2aaad3c3141cd`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:5341a778f66de392617af57df8c30ceb15ffb9962c8dd84f5855e2b5cbafefe9`,
};
