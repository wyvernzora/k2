import { stringify } from "yaml";

export interface AuthenticatedSourceIpPolicyProps {
  readonly allowCidrs?: string[];
  readonly denyCidrs?: string[];
}

export interface AuthenticatedMcpToolDenyPolicyProps extends AuthenticatedSourceIpPolicyProps {
  readonly deniedTools: string[];
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

export function authenticatedMcpToolDenyPolicy(props: AuthenticatedMcpToolDenyPolicyProps): string {
  const rules: PolicyRule[] = [];
  if (props.denyCidrs !== undefined && props.denyCidrs.length > 0) {
    rules.push({
      deny: {
        or: [{ source_ip: props.denyCidrs }],
      },
    });
  }

  if (props.deniedTools.length > 0) {
    rules.push({
      deny: {
        or: [mcpToolIn(props.deniedTools)],
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

type PolicyCriterion =
  | { readonly authenticated_user: true }
  | { readonly source_ip: string[] }
  | { readonly mcp_tool: { readonly in: string[] } };

function allowCriteria(allowCidrs: string[]): PolicyCriterion[] {
  const criteria: PolicyCriterion[] = [{ authenticated_user: true }];
  if (allowCidrs.length > 0) {
    criteria.push({ source_ip: allowCidrs });
  }
  return criteria;
}

function mcpToolIn(tools: string[]): PolicyCriterion {
  return { mcp_tool: { in: tools } };
}
