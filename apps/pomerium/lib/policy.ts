import { stringify } from "yaml";

export interface AuthenticatedSourceIpPolicyProps {
  readonly allowCidrs?: string[];
  readonly denyCidrs?: string[];
}

export function authenticatedSourceIpPolicy(props: AuthenticatedSourceIpPolicyProps = {}): string {
  const rules: PolicyRule[] = [];
  if (props.denyCidrs !== undefined && props.denyCidrs.length > 0) {
    rules.push({
      deny: {
        or: [{ source_ip: props.denyCidrs }],
      },
    });
  }

  rules.push({
    allow: {
      and: allowCriteria(props.allowCidrs ?? []),
    },
  });

  return stringify(rules);
}

interface PolicyRule {
  readonly allow?: LogicalCriteria;
  readonly deny?: LogicalCriteria;
}

interface LogicalCriteria {
  readonly and?: PolicyCriterion[];
  readonly or?: PolicyCriterion[];
}

type PolicyCriterion = { readonly authenticated_user: true } | { readonly source_ip: string[] };

function allowCriteria(allowCidrs: string[]): PolicyCriterion[] {
  const criteria: PolicyCriterion[] = [{ authenticated_user: true }];
  if (allowCidrs.length > 0) {
    criteria.push({ source_ip: allowCidrs });
  }
  return criteria;
}
