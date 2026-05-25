import type { Rule } from "eslint";

import { identifierName, isAppOrCdkSourceFile, normalizedFilename, type AstNode } from "../utils.js";

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: {
      description: "Disallow raw ApiObject when generated CRD constructs are available.",
    },
    messages: {
      rawApiObject: "Do not instantiate raw ApiObject in apps/ or cdk-lib/; use generated CRD bindings instead.",
    },
    schema: [],
  },
  create(context) {
    if (!isAppOrCdkSourceFile(normalizedFilename(context))) {
      return {};
    }
    return {
      NewExpression(node) {
        const astNode = node as unknown as AstNode;
        const callee = astNode.callee as AstNode | undefined;
        if (identifierName(callee) === "ApiObject") {
          context.report({ node, messageId: "rawApiObject" });
        }
      },
    };
  },
};

export default rule;
