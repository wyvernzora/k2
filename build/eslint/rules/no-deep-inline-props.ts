import type { Rule } from "eslint";

import { isAppOrCdkSourceFile, lineSpan, normalizedFilename, objectDepth, type AstNode } from "../utils.js";

const defaultMaxDepth = 3;
const defaultLargeConstructLines = 30;

const rule: Rule.RuleModule = {
  meta: {
    type: "suggestion",
    docs: {
      description: "Require deeply nested object literals to move into helper functions.",
    },
    messages: {
      tooDeep:
        "Object literal is nested {{depth}} levels deep; move this object construction into a named helper function in the same file.",
      tooDeepAndLarge:
        "Object literal is nested {{depth}} levels deep and the construct instantiation spans {{lines}} lines; move it into a named custom construct class extending the original construct.",
    },
    schema: [
      {
        type: "object",
        properties: {
          maxDepth: { type: "number", minimum: 1 },
          largeConstructLines: { type: "number", minimum: 1 },
        },
        additionalProperties: false,
      },
    ],
  },
  create(context) {
    if (!isAppOrCdkSourceFile(normalizedFilename(context))) {
      return {};
    }
    const options = (context.options[0] ?? {}) as { maxDepth?: number; largeConstructLines?: number };
    const maxDepth = options.maxDepth ?? defaultMaxDepth;
    const largeConstructLines = options.largeConstructLines ?? defaultLargeConstructLines;

    return {
      ObjectExpression(node) {
        const astNode = node as unknown as AstNode;
        if (hasObjectLiteralAncestor(astNode)) {
          return;
        }
        const depth = objectDepth(astNode);
        if (depth <= maxDepth) {
          return;
        }
        const construct = nearestConstructCall(astNode);
        const lines = construct === undefined ? 0 : lineSpan(construct);
        if (construct !== undefined && lines > largeConstructLines) {
          context.report({
            node,
            messageId: "tooDeepAndLarge",
            data: { depth: String(depth), lines: String(lines) },
          });
          return;
        }
        context.report({ node, messageId: "tooDeep", data: { depth: String(depth) } });
      },
    };
  },
};

export default rule;

function hasObjectLiteralAncestor(node: AstNode): boolean {
  let parent = node.parent;
  while (parent != null) {
    if (parent.type === "ObjectExpression") {
      return true;
    }
    parent = parent.parent;
  }
  return false;
}

function nearestConstructCall(node: AstNode): AstNode | undefined {
  let parent = node.parent;
  while (parent != null) {
    if (parent.type === "NewExpression") {
      return parent;
    }
    if (parent.type === "CallExpression" && (parent.callee as AstNode | undefined)?.type === "Super") {
      return parent;
    }
    parent = parent.parent;
  }
  return undefined;
}
