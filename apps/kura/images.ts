import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:8f354004e53212b951039ac0378477a491ef10dcef3c4d401d77d58d3cd2dc40`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:53ce2fcc77ee8ee664e901dc967888b7d13fc77a500c078af952375fd1effc60`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:79c9d9316860a9c4075e1b20c73abe6a9f938976ba4b804263fd5cb97f945ebf`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:2d9d756384361341b2df88c33dc84b91bb1c924911c91c6d026247c41cc87efd`,
};
