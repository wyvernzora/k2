import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:efd9fb20cd6ae8e3500124e865cffd3854ef182f1de936ad7557b0e3c5e96cbb`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:7b231448a04278807b7b3951e488f1c1bd7c0640118f898c338eaffc32931d56`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:1cd3a26d768eb1e82c70d7672512d5cdc7fc5645bd2903ad65675bf5b0a84763`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:b97a9f78f7c04006df36463195b8980ae5df0138deeafd4332ec23e419d06f61`,
};
