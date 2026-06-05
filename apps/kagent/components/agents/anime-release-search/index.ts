import { fileURLToPath } from "node:url";

import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { mcpServers as kuraMcpServers } from "@k2/kura";

import { ANIME_RELEASE_SEARCH_AGENT_NAME, GPT_5_4_MINI_MODEL_CONFIG_NAME } from "../../../constants.js";
import { KAgentDeclarativeAgent, mcpTool } from "../../../lib/agent.js";

const AGENT_DIR = fileURLToPath(new URL(".", import.meta.url));

export class AnimeReleaseSearchAgent extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new KAgentDeclarativeAgent(this, "agent", {
      name: ANIME_RELEASE_SEARCH_AGENT_NAME,
      description: "Search DMHY for anime releases using compact Kura context.",
      modelConfig: GPT_5_4_MINI_MODEL_CONFIG_NAME,
      rootDir: AGENT_DIR,
      tools: [
        mcpTool(kuraMcpServers.kura(), { toolNames: ["kura_resolve", "kura_aliases", "kura_show"] }),
        mcpTool(kuraMcpServers.dmhy(), { toolNames: ["search_releases"] }),
      ],
    });
  }
}
