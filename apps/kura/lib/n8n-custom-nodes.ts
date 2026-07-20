import { Capability, EnvValue, ImagePullPolicy, type ContainerProps, type Volume } from "cdk8s-plus-32";

import { oci } from "@k2/cdk-lib";

const KURA_N8N_NODES_IMAGE = oci`ghcr.io/wyvernzora/kura/n8n-nodes:v0.5.1`;

export interface N8NCustomNodesInitContainerProps {
  readonly volume: Volume;
  readonly path: string;
  readonly resources?: ContainerProps["resources"];
}

export function n8nCustomNodesInitContainer(props: N8NCustomNodesInitContainerProps): ContainerProps {
  return {
    name: "install-kura-nodes",
    image: KURA_N8N_NODES_IMAGE,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    envVariables: {
      KURA_NODES_TARGET: EnvValue.fromValue(props.path),
    },
    volumeMounts: [{ volume: props.volume, path: props.path }],
    ...(props.resources === undefined ? {} : { resources: props.resources }),
    securityContext: {
      capabilities: {
        drop: [Capability.ALL],
      },
      ensureNonRoot: false,
      readOnlyRootFilesystem: true,
    },
  };
}
