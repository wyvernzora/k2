import { VolumeMount } from "cdk8s-plus-27";
import { Construct, IConstruct } from "constructs";
import { createEphemeralVolume, EphemeralVolumeProps } from "./ephemeral";
import { createReplicatedVolume, ReplicatedVolumeProps } from "./replicated";
import { createNasVolume, NasVolumeProps } from "./nas";

/**
 * Specifies the mounting options of a specific volume.
 * Note that the volume itself may have a sub-path defined,
 * in which case sub-paths will be joined.
 */
export type MountProps = Omit<VolumeMount, "volume">;

export interface IMountable extends IConstruct {
  mount(props: MountProps): VolumeMount;
}

export type VolumeProps =
  | NasVolumeProps
  | ReplicatedVolumeProps
  | EphemeralVolumeProps;

export function createVolume(
  scope: Construct,
  id: string,
  props: VolumeProps,
): IMountable {
  switch (props.kind) {
    case "nas":
      return createNasVolume(scope, id, props);
    case "replicated":
      return createReplicatedVolume(scope, id, props);
    case "ephemeral":
      return createEphemeralVolume(scope, id, props);
  }
}
