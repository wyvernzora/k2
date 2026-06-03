import { Duration, type Cron } from "cdk8s";
import { ConcurrencyPolicy, CronJob, type CronJobProps, type IServiceAccount, type JobProps } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { Scheduling } from "../scheduling.js";

import {
  prepareScriptedWorkload,
  type ScriptedWorkloadProps,
  type ScriptedWorkloadRbacRule,
  type ScriptedWorkloadScript,
  type ScriptedWorkloadMount,
} from "./scripted-workload.js";

export type ScriptedCronJobScript = ScriptedWorkloadScript;
export type ScriptedCronJobRbacRule = ScriptedWorkloadRbacRule;
export type ScriptedCronJobMount = ScriptedWorkloadMount;

export interface ScriptedCronJobProps extends ScriptedWorkloadProps {
  readonly schedule: Cron;
  readonly activeDeadline?: CronJobProps["activeDeadline"];
  readonly timeZone?: string;
  readonly concurrencyPolicy?: ConcurrencyPolicy;
  readonly successfulJobsRetained?: number;
  readonly failedJobsRetained?: number;
  readonly startingDeadline?: CronJobProps["startingDeadline"];
}

export class ScriptedCronJob extends Construct {
  public readonly serviceAccount?: IServiceAccount;

  public constructor(scope: Construct, id: string, props: ScriptedCronJobProps) {
    super(scope, id);

    const prepared = prepareScriptedWorkload(this, props, { type: "cronjob" });
    this.serviceAccount = prepared.serviceAccount;
    new ScriptedKubernetesCronJob(this, "cron-job", scriptedCronJobProps(props, prepared.jobProps));
  }
}

class ScriptedKubernetesCronJob extends CronJob {
  public constructor(scope: Construct, id: string, props: CronJobProps) {
    super(scope, id, props);
    Scheduling.applyWorkersPreferred(this);
  }
}

function scriptedCronJobProps(props: ScriptedCronJobProps, jobProps: JobProps): CronJobProps {
  return {
    ...jobProps,
    activeDeadline: props.activeDeadline ?? Duration.minutes(10),
    ttlAfterFinished: undefined,
    schedule: props.schedule,
    timeZone: props.timeZone,
    concurrencyPolicy: props.concurrencyPolicy ?? ConcurrencyPolicy.FORBID,
    successfulJobsRetained: props.successfulJobsRetained ?? 1,
    failedJobsRetained: props.failedJobsRetained ?? 3,
    startingDeadline: props.startingDeadline,
  };
}
