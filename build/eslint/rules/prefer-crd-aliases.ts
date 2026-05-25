import type { Rule } from "eslint";

import {
  isAliasInitializer,
  isGeneratedCrdFile,
  isTopLevelMemberExpression,
  memberExpressionText,
  normalizedFilename,
  type AstNode,
} from "../utils.js";

const defaultMinLength = 20;
const defaultMinUses = 1;

const rule: Rule.RuleModule = {
  meta: {
    type: "suggestion",
    docs: {
      description: "Prefer top-of-file aliases for long generated CRD member chains.",
    },
    messages: {
      preferAlias:
        "Generated CRD member chain '{{chain}}' is too long to read inline; alias it near the top of the file.",
    },
    schema: [
      {
        type: "object",
        properties: {
          minLength: { type: "number", minimum: 1 },
          minUses: { type: "number", minimum: 1 },
        },
        additionalProperties: false,
      },
    ],
  },
  create(context) {
    if (isGeneratedCrdFile(normalizedFilename(context))) {
      return {};
    }
    const options = (context.options[0] ?? {}) as { minLength?: number; minUses?: number };
    const minLength = options.minLength ?? defaultMinLength;
    const minUses = options.minUses ?? defaultMinUses;
    const uses = new Map<string, Rule.Node[]>();

    return {
      MemberExpression(node) {
        const astNode = node as unknown as AstNode;
        if (!isTopLevelMemberExpression(astNode) || isAliasInitializer(astNode)) {
          return;
        }
        const text = memberExpressionText(astNode, context.sourceCode);
        if (text === undefined || text.length <= minLength || !looksLikeGeneratedCrdChain(text)) {
          return;
        }
        const nodes = uses.get(text) ?? [];
        nodes.push(node);
        uses.set(text, nodes);
      },
      "Program:exit"() {
        for (const [chain, nodes] of uses.entries()) {
          if (nodes.length >= minUses) {
            context.report({
              node: nodes[0],
              messageId: "preferAlias",
              data: { chain },
            });
          }
        }
      },
    };
  },
};

export default rule;

function looksLikeGeneratedCrdChain(text: string): boolean {
  return /^crd\.[A-Z][A-Za-z0-9]+/.test(text) || /\b[A-Z][A-Za-z0-9]+Spec[A-Za-z0-9]+\./.test(text);
}
