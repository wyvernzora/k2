import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:da8854b8bff7afbe811be550125bd22663f7b0b10074572437a49e72073cca38`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:e441e178a6d629b01358e6c00147548b2e38e66b6d9f4808e8149c99ce52d758`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:af82626d59a5525cfa36b43239126ee9a7f99c85a38b34de9dd2b4d51997f555`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:16e17931ead93c9f7b30fbd1ab8a9d6e279ab88369fff51abcd30e5f0d89c8cd`,
};
