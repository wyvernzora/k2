import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:c7df127762d08af10222e2de513bc97521cb3f429eabb7f2c20676c0e8b08fbb`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:62fdb6f99718a18376b88ce3298294198dd1f1f2c30787578429402e37a1479b`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:d932c1c42b27a99fbcb2f44e35308682543084ba2c3f56d2c7e8be6a0e6f6749`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:e0fc6f5948e49d0ddb9cc86760cad37416db4068e754dbe2afd330663e7f68e1`,
};
