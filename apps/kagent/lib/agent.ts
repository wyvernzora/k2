import { readFileSync, readdirSync } from "node:fs";
import { basename, join } from "node:path";

import { ConfigMap } from "cdk8s-plus-32";
import { Construct } from "constructs";
import { parse } from "yaml";

import { PrivateConnection } from "@k2/cilium";
import type { PolicyEndpoint, PrivateConnectionTarget } from "@k2/cilium";

import {
  AgentV1Alpha2,
  AgentV1Alpha2SpecDeclarativeSystemMessageFromType,
  AgentV1Alpha2SpecDeclarativeToolsType,
  AgentV1Alpha2SpecType,
  type AgentV1Alpha2SpecDeclarativeA2AConfigSkills,
  type AgentV1Alpha2SpecDeclarativeTools,
} from "../crds/kagent.dev.js";

const AgentType = AgentV1Alpha2SpecType;
const SystemMessageFromType = AgentV1Alpha2SpecDeclarativeSystemMessageFromType;
const ToolType = AgentV1Alpha2SpecDeclarativeToolsType;

const KAGENT_API_GROUP = "kagent.dev";
const PROMPT_FILENAME = "prompt.md";
const SKILLS_DIRNAME = "skills";
const SYSTEM_PROMPT_KEY = "system-prompt";

export interface McpServer {
  readonly name: string;
  readonly description: string;
  readonly url: string;
  readonly connection: PrivateConnectionTarget;
  readonly toolNames: readonly string[];
  readonly apiGroup?: string;
  readonly kind?: string;
  readonly namespace?: string;
  readonly allowedHeaders?: readonly string[];
}

export interface KAgentDeclarativeAgentProps {
  readonly name: string;
  readonly description: string;
  readonly modelConfig: string;
  readonly rootDir: string;
  readonly tools: readonly KAgentTool[];
  readonly stream?: boolean;
  readonly network?: KAgentDeclarativeAgentNetwork;
}

export interface KAgentDeclarativeAgentNetwork {
  readonly mcpClient?: PolicyEndpoint;
}

export type KAgentTool = KAgentMcpTool | KAgentAgentTool | AgentV1Alpha2SpecDeclarativeTools;

export interface KAgentMcpTool {
  readonly type: "mcp";
  readonly server: McpServer;
  readonly toolNames?: readonly string[];
  readonly requireApproval?: readonly string[];
}

export interface KAgentAgentTool {
  readonly type: "agent";
  readonly name: string;
  readonly apiGroup?: string;
  readonly kind?: string;
  readonly namespace?: string;
}

interface SkillFrontMatter {
  readonly name?: unknown;
  readonly examples?: unknown;
  readonly inputModes?: unknown;
  readonly outputModes?: unknown;
  readonly tags?: unknown;
}

export class KAgentDeclarativeAgent extends Construct {
  public constructor(scope: Construct, id: string, props: KAgentDeclarativeAgentProps) {
    super(scope, id);

    const promptConfigMapName = `${props.name}-system-prompt`;

    new ConfigMap(this, "system-prompt", {
      metadata: { name: promptConfigMapName },
      data: { [SYSTEM_PROMPT_KEY]: readFileSync(join(props.rootDir, PROMPT_FILENAME), "utf8") },
    });
    new AgentV1Alpha2(this, "agent", {
      metadata: { name: props.name },
      spec: agentSpec(props, promptConfigMapName),
    });
    createNetworkPolicies(this, props);
  }
}

export function mcpTool(server: McpServer, props: Omit<KAgentMcpTool, "type" | "server"> = {}): KAgentMcpTool {
  return { type: "mcp", server, ...props };
}

export function agentTool(name: string, props: Omit<KAgentAgentTool, "type" | "name"> = {}): KAgentAgentTool {
  return { type: "agent", name, ...props };
}

function renderTool(tool: KAgentTool): AgentV1Alpha2SpecDeclarativeTools {
  if (tool.type === "mcp") {
    return renderMcpTool(tool);
  }
  if (tool.type === "agent") {
    return renderAgentTool(tool);
  }
  return tool;
}

function agentSpec(props: KAgentDeclarativeAgentProps, promptConfigMapName: string) {
  return {
    description: props.description,
    type: AgentType.DECLARATIVE,
    declarative: {
      a2AConfig: { skills: loadSkills(props.rootDir) },
      modelConfig: props.modelConfig,
      stream: props.stream ?? true,
      systemMessageFrom: systemMessageFrom(promptConfigMapName),
      tools: props.tools.map(renderTool),
    },
  };
}

function systemMessageFrom(promptConfigMapName: string) {
  return {
    name: promptConfigMapName,
    key: SYSTEM_PROMPT_KEY,
    type: SystemMessageFromType.CONFIG_MAP,
  };
}

function renderMcpTool(tool: KAgentMcpTool): AgentV1Alpha2SpecDeclarativeTools {
  return {
    type: ToolType.MCP_SERVER,
    mcpServer: {
      allowedHeaders: tool.server.allowedHeaders === undefined ? undefined : [...tool.server.allowedHeaders],
      apiGroup: tool.server.apiGroup ?? KAGENT_API_GROUP,
      kind: tool.server.kind ?? "RemoteMCPServer",
      name: tool.server.name,
      namespace: tool.server.namespace,
      requireApproval: tool.requireApproval === undefined ? undefined : [...tool.requireApproval],
      toolNames: [...(tool.toolNames ?? tool.server.toolNames)],
    },
  };
}

function renderAgentTool(tool: KAgentAgentTool): AgentV1Alpha2SpecDeclarativeTools {
  return {
    type: ToolType.AGENT,
    agent: {
      apiGroup: tool.apiGroup ?? KAGENT_API_GROUP,
      kind: tool.kind ?? "Agent",
      name: tool.name,
      namespace: tool.namespace,
    },
  };
}

function createNetworkPolicies(scope: Construct, props: KAgentDeclarativeAgentProps): void {
  if (props.network?.mcpClient === undefined) {
    return;
  }

  for (const server of uniqueMcpServers(props.tools)) {
    new PrivateConnection(scope, `to-${server.name}`, {
      from: props.network.mcpClient,
      ...server.connection,
    });
  }
}

function uniqueMcpServers(tools: readonly KAgentTool[]): McpServer[] {
  const servers = new Map<string, McpServer>();
  for (const tool of tools) {
    if (tool.type === "mcp") {
      servers.set(tool.server.name, tool.server);
    }
  }
  return [...servers.values()];
}

function loadSkills(rootDir: string): AgentV1Alpha2SpecDeclarativeA2AConfigSkills[] {
  const skillsDir = join(rootDir, SKILLS_DIRNAME);
  return readdirSync(skillsDir, { withFileTypes: true })
    .filter(entry => entry.isFile() && entry.name.endsWith(".md"))
    .map(entry => entry.name)
    .sort()
    .map(filename => loadSkill(join(skillsDir, filename)));
}

function loadSkill(path: string): AgentV1Alpha2SpecDeclarativeA2AConfigSkills {
  const id = basename(path, ".md");
  const { frontMatter, body } = splitMarkdown(path);
  return {
    id,
    name: requiredString(frontMatter.name, `${path}: front matter field "name"`),
    description: requiredBody(body, path),
    examples: optionalStringArray(frontMatter.examples, `${path}: front matter field "examples"`),
    inputModes: optionalStringArray(frontMatter.inputModes, `${path}: front matter field "inputModes"`) ?? ["text"],
    outputModes: optionalStringArray(frontMatter.outputModes, `${path}: front matter field "outputModes"`) ?? ["text"],
    tags: optionalStringArray(frontMatter.tags, `${path}: front matter field "tags"`) ?? [],
  };
}

function splitMarkdown(path: string): { readonly frontMatter: SkillFrontMatter; readonly body: string } {
  const content = readFileSync(path, "utf8");
  const match = /^---\n([\s\S]*?)\n---\n?([\s\S]*)$/.exec(content);
  if (match === null) {
    throw new Error(`${path} must start with YAML front matter`);
  }

  const frontMatter = parse(match[1]) as unknown;
  if (!isObject(frontMatter)) {
    throw new Error(`${path} front matter must be a YAML object`);
  }
  return { frontMatter, body: match[2] };
}

function requiredString(value: unknown, field: string): string {
  const result = optionalString(value, field);
  if (result === undefined) {
    throw new Error(`${field} is required`);
  }
  return result;
}

function requiredBody(body: string, path: string): string {
  const result = body.trim();
  if (result.length === 0) {
    throw new Error(`${path} must include a markdown body for the skill description`);
  }
  return result;
}

function optionalString(value: unknown, field: string): string | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (typeof value !== "string" || value.length === 0) {
    throw new Error(`${field} must be a non-empty string`);
  }
  return value;
}

function optionalStringArray(value: unknown, field: string): string[] | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!Array.isArray(value) || value.some(item => typeof item !== "string" || item.length === 0)) {
    throw new Error(`${field} must be a list of non-empty strings`);
  }
  return [...value];
}

function isObject(value: unknown): value is SkillFrontMatter {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
