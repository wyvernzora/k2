import type { Rule } from "eslint";

import { isCdkLibFile, normalizedFilename, sourceValue, type ImportLikeNode } from "../utils.js";

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: {
      description: "Keep cdk-lib app-agnostic by disallowing app imports.",
    },
    messages: {
      appImport: "cdk-lib must stay app-agnostic; do not import app packages or app CRD paths here.",
    },
    schema: [],
  },
  create(context) {
    if (!isCdkLibFile(normalizedFilename(context))) {
      return {};
    }
    return {
      ImportDeclaration(node) {
        const source = sourceValue(node as unknown as ImportLikeNode);
        if (source !== undefined && isForbiddenAppImport(source)) {
          context.report({ node, messageId: "appImport" });
        }
      },
    };
  },
};

export default rule;

function isForbiddenAppImport(source: string): boolean {
  if (/^@k2\/(?!cdk-lib$)[^/]+/.test(source)) {
    return true;
  }
  return source.includes("/apps/") || source.startsWith("apps/") || source.startsWith("../apps/");
}
