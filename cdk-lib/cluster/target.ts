export const CLUSTER_TARGETS = ["legacy"] as const;
export type ClusterTarget = (typeof CLUSTER_TARGETS)[number];
