export interface TopologySpreadOptions {
  readonly matchLabels: Record<string, string>;
  readonly maxSkew?: number;
  readonly whenUnsatisfiable?: "DoNotSchedule" | "ScheduleAnyway";
}

export interface TopologySpreadConstraint {
  readonly maxSkew: number;
  readonly topologyKey: string;
  readonly whenUnsatisfiable: "DoNotSchedule" | "ScheduleAnyway";
  readonly labelSelector: {
    readonly matchLabels: Record<string, string>;
  };
}

const DEFAULT_MAX_SKEW = 1;
const DEFAULT_WHEN_UNSATISFIABLE = "ScheduleAnyway";

export const TopologySpread = {
  acrossZones(options: TopologySpreadOptions): TopologySpreadConstraint {
    return topologySpreadConstraint("topology.kubernetes.io/zone", options);
  },

  acrossHosts(options: TopologySpreadOptions): TopologySpreadConstraint {
    return topologySpreadConstraint("kubernetes.io/hostname", options);
  },
};

function topologySpreadConstraint(topologyKey: string, options: TopologySpreadOptions): TopologySpreadConstraint {
  return {
    maxSkew: options.maxSkew ?? DEFAULT_MAX_SKEW,
    topologyKey,
    whenUnsatisfiable: options.whenUnsatisfiable ?? DEFAULT_WHEN_UNSATISFIABLE,
    labelSelector: {
      matchLabels: options.matchLabels,
    },
  };
}
