import { Capability, EnvValue, ImagePullPolicy, type ContainerProps, type Volume } from "cdk8s-plus-32";

import { KURA_IMAGES } from "../images.js";

export interface N8NCustomNodesInitContainerProps {
  readonly volume: Volume;
  readonly path: string;
  readonly resources?: ContainerProps["resources"];
}

export function n8nCustomNodesInitContainer(props: N8NCustomNodesInitContainerProps): ContainerProps {
  return {
    name: "install-kura-nodes",
    image: KURA_IMAGES.n8nNodes,
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
