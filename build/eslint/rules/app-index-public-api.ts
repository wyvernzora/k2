import type { Rule } from "eslint";

import {
  exportedDeclarationNames,
  hasTypeAnnotation,
  isAppIndexFile,
  normalizedFilename,
  sourceValue,
  type AstNode,
  type ImportLikeNode,
} from "../utils.js";

const allowedLocalExportTypes = new Map([
  ["configureArgoApplication", "ArgoApplicationConfigFunc"],
  ["createAppResources", "AppResourceFunc"],
]);

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: {
      description: "Require app index files to expose only createAppResources and lib re-exports.",
    },
    messages: {
      nonLibReExport: "App index files may only re-export public API from ./lib/.",
      unexpectedExport:
        "App index files may only export createAppResources, configureArgoApplication, and re-export from ./lib/.",
      missingType: "App index public API constants must use their required K2 type annotation.",
    },
    schema: [],
  },
  create(context) {
    if (!isAppIndexFile(normalizedFilename(context))) {
      return {};
    }

    return {
      ExportAllDeclaration(node) {
        validateReExport(context, node);
      },
      ExportNamedDeclaration(node) {
        const astNode = node as unknown as AstNode & ImportLikeNode;
        const source = sourceValue(astNode);
        if (source !== undefined) {
          validateReExport(context, node);
          return;
        }

        const names = exportedDeclarationNames(astNode);
        if (names.length === 0) {
          context.report({ node, messageId: "unexpectedExport" });
          return;
        }

        for (const name of names) {
          const requiredType = allowedLocalExportTypes.get(name);
          if (requiredType === undefined) {
            context.report({ node, messageId: "unexpectedExport" });
            continue;
          }
          if (!hasTypeAnnotation(astNode, context.sourceCode, requiredType)) {
            context.report({ node, messageId: "missingType" });
          }
        }
      },
      ExportDefaultDeclaration(node) {
        context.report({ node, messageId: "unexpectedExport" });
      },
    };
  },
};

export default rule;

function validateReExport(context: Rule.RuleContext, node: unknown): void {
  const source = sourceValue(node as ImportLikeNode);
  if (source === undefined || !source.startsWith("./lib/")) {
    context.report({ node: node as never, messageId: "nonLibReExport" });
  }
}
