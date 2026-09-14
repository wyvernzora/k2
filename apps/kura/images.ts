import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:1bf4a5ed8cf41743ed0eea9dc2d4a65a1c9210dd0a127ae99d82cc27195a74d6`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:9bc0220f71af465a2398900eb3f308e778cc65250bdb57d39be4f8e58cbe4461`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:dcfca0d137877c7114bf7eba4a938fd6b2bd606a296a87f6375b48568cf9e4c2`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:adb711dfa10f8c941c4d531d69f31c68d8aabf2b4b14a8604581c1cef42b1863`,
};
