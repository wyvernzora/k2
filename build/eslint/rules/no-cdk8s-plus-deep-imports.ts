import type { Rule } from "eslint";

import { isAppOrCdkSourceFile, normalizedFilename, sourceValue, type ImportLikeNode } from "../utils.js";

const RULE_NAME = "no-cdk8s-plus-deep-imports";
const K2_RULE_NAME = `k2/${RULE_NAME}`;

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: {
      description: "Require cdk8s-plus imports to use top-level package exports.",
    },
    messages: {
      deepImport:
        "Do not import from cdk8s-plus subpaths; import from the top-level cdk8s-plus package export instead.",
      missingDisableReason: `Disabling ${K2_RULE_NAME} requires a "-- reason" description.`,
    },
    schema: [],
  },
  create(context) {
    if (!isAppOrCdkSourceFile(normalizedFilename(context))) {
      return {};
    }
    return {
      Program() {
        for (const comment of context.sourceCode.getAllComments()) {
          if (isTargetDisableComment(comment.value) && !hasDisableReason(comment.value)) {
            const loc = comment.loc;
            if (loc !== undefined && loc !== null) {
              context.report({ loc, messageId: "missingDisableReason" });
            }
          }
        }
      },
      ImportDeclaration(node) {
        const source = sourceValue(node as unknown as ImportLikeNode);
        if (source !== undefined && isCdk8sPlusDeepImport(source)) {
          context.report({ node, messageId: "deepImport" });
        }
      },
    };
  },
};

export default rule;

function isCdk8sPlusDeepImport(source: string): boolean {
  return /^cdk8s-plus-[^/]+\/.+/.test(source);
}

function isTargetDisableComment(value: string): boolean {
  return /\beslint-disable(?:-next-line|-line)?\b/.test(value) && value.includes(`/${RULE_NAME}`);
}

function hasDisableReason(value: string): boolean {
  return /--\s+\S/.test(value);
}
