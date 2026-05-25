import type { Rule } from "eslint";

import { componentHasIndex, componentPath, nonBlankLineCount, normalizedFilename } from "../utils.js";

const defaultMaxSingleFileSloc = 100;

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: {
      description: "Require app component direct children to be simple files or component directories.",
    },
    messages: {
      missingIndex: "Component directory {{name}} must provide an index.ts facade.",
      tooLarge:
        "Single-file component {{name}} has {{lines}} SLOC; move it to components/{{name}}/index.ts with neighbors.",
    },
    schema: [
      {
        type: "object",
        properties: {
          maxSingleFileSloc: { type: "number", minimum: 1 },
        },
        additionalProperties: false,
      },
    ],
  },
  create(context) {
    const filename = normalizedFilename(context);
    const info = componentPath(filename);
    if (info === undefined) {
      return {};
    }
    const options = (context.options[0] ?? {}) as { maxSingleFileSloc?: number };
    const maxSingleFileSloc = options.maxSingleFileSloc ?? defaultMaxSingleFileSloc;

    return {
      Program(node) {
        if (info.rest.length === 0 && filename.endsWith(".ts")) {
          const lines = nonBlankLineCount(context.sourceCode);
          if (lines > maxSingleFileSloc) {
            context.report({
              node,
              messageId: "tooLarge",
              data: { name: info.componentName.replace(/\.ts$/, ""), lines: String(lines) },
            });
          }
          return;
        }

        if (info.rest.length > 0 && !componentHasIndex(info.componentRoot)) {
          context.report({ node, messageId: "missingIndex", data: { name: info.componentName } });
        }
      },
    };
  },
};

export default rule;
