import { ApplicationSpecIgnoreDifferences as IgnoreDifferences } from "@k2/argocd/crds";

export interface DeployOptions {
  readonly namespace?: string;
  readonly autoSync?: boolean;
  readonly ignoreDifferences?: IgnoreDifferences[];
}

export const DefaultDeployOptions: DeployOptions = {
  autoSync: true,
  ignoreDifferences: [],
};
