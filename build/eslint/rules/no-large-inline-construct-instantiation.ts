import type { Rule } from "eslint";

import { isAppOrCdkSourceFile, lineSpan, normalizedFilename, type AstNode } from "../utils.js";

const defaultMaxLines = 30;

const rule: Rule.RuleModule = {
  meta: {
    type: "suggestion",
    docs: {
      description: "Require large inline construct instantiations to move into named construct classes.",
    },
    messages: {
      tooLarge:
        "Inline construct instantiation spans {{lines}} lines; move it into a named custom construct class extending the original construct.",
    },
    schema: [
      {
        type: "object",
        properties: {
          maxLines: { type: "number", minimum: 1 },
        },
        additionalProperties: false,
      },
    ],
  },
  create(context) {
    if (!isAppOrCdkSourceFile(normalizedFilename(context))) {
      return {};
    }
    const options = (context.options[0] ?? {}) as { maxLines?: number };
    const maxLines = options.maxLines ?? defaultMaxLines;

    return {
      NewExpression(node) {
        const lines = lineSpan(node as unknown as AstNode);
        if (lines > maxLines) {
          context.report({ node, messageId: "tooLarge", data: { lines: String(lines) } });
        }
      },
    };
  },
};

export default rule;
