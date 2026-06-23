import type { ApiObjectMetadata } from "cdk8s";

const ARGO_SYNC_OPTIONS_ANNOTATION = "argocd.argoproj.io/sync-options";
const REPLACEABLE_JOB_SYNC_OPTIONS = ["Force=true", "Replace=true"] as const;

export function withReplaceableJobSyncOptions(metadata: ApiObjectMetadata): ApiObjectMetadata {
  return {
    ...metadata,
    annotations: {
      ...metadata.annotations,
      [ARGO_SYNC_OPTIONS_ANNOTATION]: mergedSyncOptions(metadata.annotations?.[ARGO_SYNC_OPTIONS_ANNOTATION]),
    },
  };
}

function mergedSyncOptions(existing: string | undefined): string {
  const options = existingOptions(existing);
  for (const option of REPLACEABLE_JOB_SYNC_OPTIONS) {
    if (!options.includes(option)) {
      options.push(option);
    }
  }
  return options.join(",");
}

function existingOptions(existing: string | undefined): string[] {
  return (
    existing
      ?.split(",")
      .map(option => option.trim())
      .filter(option => option !== "") ?? []
  );
}
