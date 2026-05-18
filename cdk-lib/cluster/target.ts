export const CLUSTER_TARGETS = ["legacy", "v3"] as const;
export type ClusterTarget = (typeof CLUSTER_TARGETS)[number];
