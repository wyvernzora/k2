import { Capability, EnvValue, ImagePullPolicy, type ContainerProps, type Volume } from "cdk8s-plus-32";

const QBIT_BRIDGE_N8N_NODES_IMAGE = "ghcr.io/wyvernzora/qbit-bridge/n8n-nodes:dev";

export interface N8NCustomNodesInitContainerProps {
  readonly volume: Volume;
  readonly path: string;
  readonly resources?: ContainerProps["resources"];
}

export function n8nCustomNodesInitContainer(props: N8NCustomNodesInitContainerProps): ContainerProps {
  return {
    name: "install-qbit-bridge-nodes",
    image: QBIT_BRIDGE_N8N_NODES_IMAGE,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    envVariables: {
      QBIT_BRIDGE_NODES_TARGET: EnvValue.fromValue(props.path),
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
