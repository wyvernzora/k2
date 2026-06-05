import { ConfigMap } from "cdk8s-plus-32";
import dedent from "dedent-js";
import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";

import {
  AgentV1Alpha2,
  AgentV1Alpha2SpecDeclarativeSystemMessageFromType,
  AgentV1Alpha2SpecDeclarativeToolsType,
  AgentV1Alpha2SpecType,
  RemoteMcpServer,
  RemoteMcpServerSpecProtocol,
} from "../../crds/kagent.dev.js";

const AgentType = AgentV1Alpha2SpecType;
const McpProtocol = RemoteMcpServerSpecProtocol;
const SystemMessageFromType = AgentV1Alpha2SpecDeclarativeSystemMessageFromType;
const ToolType = AgentV1Alpha2SpecDeclarativeToolsType;

const AGENT_NAME = "anime-kura-agent";
const KURA_MCP_SERVER_NAME = "kura-mcp";
const SYSTEM_PROMPT_CONFIG_MAP_NAME = `${AGENT_NAME}-system-prompt`;
const SYSTEM_PROMPT_KEY = "system-prompt";
const KAGENT_API_GROUP = "kagent.dev";
const DEFAULT_MODEL_CONFIG_NAME = "default-model-config";
const KURA_MCP_URL = "http://kura-mcp.kura.svc.cluster.local/mcp";
const KURA_RECONCILE_APPLY_TOOL = "kura_reconcile_apply";

const KURA_TOOL_NAMES = [
  "kura_resolve",
  "kura_aliases",
  "kura_list",
  "kura_show",
  "kura_inbox_list",
  "kura_job_status",
  "kura_reconcile_plan",
  "kura_add",
  "kura_import",
  "kura_scan",
  "kura_stage",
  "kura_reset",
  KURA_RECONCILE_APPLY_TOOL,
];

export class AnimeKuraAgent extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new ConfigMap(this, "system-prompt", {
      metadata: { name: SYSTEM_PROMPT_CONFIG_MAP_NAME },
      data: { [SYSTEM_PROMPT_KEY]: systemPrompt() },
    });
    new RemoteMcpServer(this, "kura-mcp-server", {
      metadata: { name: KURA_MCP_SERVER_NAME },
      spec: {
        description: "Kura anime library MCP server.",
        protocol: McpProtocol.STREAMABLE_UNDERSCORE_HTTP,
        sseReadTimeout: "5m",
        terminateOnClose: true,
        timeout: "60s",
        url: KURA_MCP_URL,
      },
    });
    new AgentV1Alpha2(this, "anime-kura-agent", {
      metadata: { name: AGENT_NAME },
      spec: animeKuraAgentSpec(),
    });
  }
}

function animeKuraAgentSpec() {
  return {
    description: "Manage the Kura anime library through Kura MCP tools.",
    type: AgentType.DECLARATIVE,
    declarative: {
      a2AConfig: a2AConfig(),
      modelConfig: DEFAULT_MODEL_CONFIG_NAME,
      stream: true,
      systemMessageFrom: systemMessageFrom(),
      tools: [kuraMcpTool()],
    },
  };
}

function a2AConfig() {
  return {
    skills: [
      {
        id: "anime-library-management",
        name: "Anime library management",
        description:
          "Resolve anime titles, inspect Kura library state, adopt inbox files, stage changes, scan series, and plan or apply reconciles.",
        examples: [
          "List incomplete anime",
          "Show inbox files that can be adopted",
          "Plan a reconcile for a resolved series",
        ],
        inputModes: ["text"],
        outputModes: ["text"],
        tags: ["anime", "kura", "library"],
      },
    ],
  };
}

function systemMessageFrom() {
  return {
    name: SYSTEM_PROMPT_CONFIG_MAP_NAME,
    key: SYSTEM_PROMPT_KEY,
    type: SystemMessageFromType.CONFIG_MAP,
  };
}

function kuraMcpTool() {
  return {
    type: ToolType.MCP_SERVER,
    mcpServer: {
      apiGroup: KAGENT_API_GROUP,
      kind: "RemoteMCPServer",
      name: KURA_MCP_SERVER_NAME,
      toolNames: KURA_TOOL_NAMES,
      requireApproval: [KURA_RECONCILE_APPLY_TOOL],
    },
  };
}

function systemPrompt(): string {
  return dedent`
    You manage an anime library through Kura MCP tools.

    Kura is the source of truth for tracked series, inbox files, staged changes,
    and reconcile state. Use Kura tools to resolve titles, inspect library state,
    adopt inbox files, stage changes, scan series, plan reconciles, apply reconciles,
    and poll async jobs.

    Rules:
    - Always resolve titles to a MetadataRef before acting. Never invent refs.
    - If resolve returns 0 matches, surface that. If it returns 2 or more, ask the user to disambiguate.
    - Read-only tools are safe to call freely: resolve, list, show, inbox_list, reconcile_plan, job_status.
    - Before any mutation, have a clear one-sentence reason for the action.
    - For async tools, poll kura_job_status until succeeded, failed, or cancelled.
    - Before reconcile_apply, inspect the reconcile plan and summarize what will move, replace, or be trashed.
    - Copy inbox: and series: selectors exactly from tool output.
    - Preserve companion subtitle files when staging episode files.
    - Infer source labels conservatively from release/title tokens. Ask if the result would otherwise be Unknown.
    - For in-place source relabeling, use series: paths, set replace: true, and omit companions.
    - You cannot search releases, manage torrents, or control qBittorrent in this MVP.
  `;
}
