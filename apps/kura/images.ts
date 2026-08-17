import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:f6e0d5506b95f4a7182dfdc5a9829bd23438e8699bd627acd5875e98d83ceaf9`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:10702fd41a066394a897f43e81b8e8736a8f4bf45d45519ebc5c1c05402bee82`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:c1c27c25d44806ffaddc707564dc1f0c220ba41976f80754a794a2b6c02f0609`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:2509c5fd557c6e8fc791267b3c6f60263b3b0a8bfd8ff8ac57747e9ebfe1a911`,
};
