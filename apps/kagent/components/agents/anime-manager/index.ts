import { fileURLToPath } from "node:url";

import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { mcpServers as kuraMcpServers } from "@k2/kura";
import { mcpServers as qbitBridgeMcpServers } from "@k2/qbittorrent";

import {
  ANIME_MANAGER_AGENT_NAME,
  ANIME_RELEASE_SEARCH_AGENT_NAME,
  GPT_5_5_MODEL_CONFIG_NAME,
} from "../../../constants.js";
import { KAgentDeclarativeAgent, agentTool, mcpTool } from "../../../lib/agent.js";

const AGENT_DIR = fileURLToPath(new URL(".", import.meta.url));
const KURA_RECONCILE_APPLY_TOOL = "kura_reconcile_apply";
const QBIT_ADD_DOWNLOAD_TOOL = "qbit_add_download";
const QBIT_REMOVE_DOWNLOADS_TOOL = "qbit_remove_downloads";

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

export class AnimeManagerAgent extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new KAgentDeclarativeAgent(this, "agent", {
      name: ANIME_MANAGER_AGENT_NAME,
      description: "Manage the Kura anime library, release search, and qBittorrent download flow.",
      modelConfig: GPT_5_5_MODEL_CONFIG_NAME,
      rootDir: AGENT_DIR,
      tools: [
        mcpTool(kuraMcpServers.kura(), {
          toolNames: KURA_TOOL_NAMES,
          requireApproval: [KURA_RECONCILE_APPLY_TOOL],
        }),
        mcpTool(kuraMcpServers.dmhy(), { toolNames: ["get_magnets"] }),
        mcpTool(qbitBridgeMcpServers.qbitBridge(), {
          toolNames: [QBIT_ADD_DOWNLOAD_TOOL, "qbit_search_downloads", QBIT_REMOVE_DOWNLOADS_TOOL],
          requireApproval: [QBIT_ADD_DOWNLOAD_TOOL, QBIT_REMOVE_DOWNLOADS_TOOL],
        }),
        agentTool(ANIME_RELEASE_SEARCH_AGENT_NAME),
      ],
    });
  }
}
