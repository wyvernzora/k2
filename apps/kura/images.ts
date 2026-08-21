import { oci } from "@k2/cdk-lib";

// One deployment tuple: after Kura's publish workflow succeeds, replace all
// four main digests together and run `earthly +kura-image-suite` before commit.
export const KURA_IMAGES = {
  libraryManager: oci`ghcr.io/wyvernzora/kura/library-manager:main@sha256:50cd8d8f383216c924ea9ebfd1a42843b3d2f7e7f32e0709c230f894c902f5bd`,
  gateway: oci`ghcr.io/wyvernzora/kura/gateway:main@sha256:b66114c7a3541f4d9908245e4fc5575a5a55b381ceb723187fa730b094236bac`,
  releaseIndexer: oci`ghcr.io/wyvernzora/kura/release-indexer:main@sha256:8cd65216cc194119b04eefa1eeb8418fe71a2aacb6dc57d754d3229101edbf73`,
  n8nNodes: oci`ghcr.io/wyvernzora/kura/n8n-nodes:main@sha256:7ed3c4001f76ad85060eea37031d2642286a2d574300e32d6cd464aa8cb80bb0`,
};
