/* eslint-disable k2/no-raw-k8s-jobs -- K2 ScriptedJob owns the allowed raw cdk8s-plus Job construction layer. */
import type { Duration } from "cdk8s";
import { Job, type IServiceAccount, type JobProps } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { only, Scheduling, workers } from "../scheduling.js";

import { withReplaceableJobSyncOptions } from "./replaceable-job.js";
import {
  prepareScriptedWorkload,
  type ScriptedWorkloadProps,
  type ScriptedWorkloadRbacRule,
  type ScriptedWorkloadScript,
  type ScriptedWorkloadMount,
} from "./scripted-workload.js";

export type ScriptedJobScript = ScriptedWorkloadScript;
export type ScriptedJobRbacRule = ScriptedWorkloadRbacRule;
export type ScriptedJobMount = ScriptedWorkloadMount;

export interface ScriptedJobProps extends ScriptedWorkloadProps {
  readonly replaceOnChange?: boolean;
  readonly ttlAfterFinished?: Duration;
}

export class ScriptedJob extends Construct {
  public readonly job: Job;
  public readonly serviceAccount?: IServiceAccount;

  public constructor(scope: Construct, id: string, props: ScriptedJobProps) {
    super(scope, id);

    const prepared = prepareScriptedWorkload(this, props, { type: "job" });
    this.serviceAccount = prepared.serviceAccount;
    this.job = new ScriptedKubernetesJob(this, "job", scriptedJobProps(props, prepared.jobProps));
  }
}

class ScriptedKubernetesJob extends Job {
  public constructor(scope: Construct, id: string, props: JobProps) {
    super(scope, id, props);
    Scheduling.of(this).apply(only(workers()));
  }
}

function scriptedJobProps(props: ScriptedJobProps, jobProps: JobProps): JobProps {
  return {
    ...jobProps,
    metadata:
      props.replaceOnChange === false ? jobProps.metadata : withReplaceableJobSyncOptions(jobProps.metadata ?? {}),
    ttlAfterFinished: props.ttlAfterFinished,
  };
}
