import type { Rule } from "eslint";

import {
  exportedDeclarationNames,
  hasAppResourceFuncAnnotation,
  isAppIndexFile,
  normalizedFilename,
  sourceValue,
  type AstNode,
  type ImportLikeNode,
} from "../utils.js";

const allowedLocalExportNames = new Set(["createAppResources"]);

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: {
      description: "Require app index files to expose only createAppResources and lib re-exports.",
    },
    messages: {
      nonLibReExport: "App index files may only re-export public API from ./lib/.",
      unexpectedExport: "App index files may only export createAppResources and re-export from ./lib/.",
      missingType: "createAppResources must be exported as a typed AppResourceFunc constant.",
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
          if (!allowedLocalExportNames.has(name)) {
            context.report({ node, messageId: "unexpectedExport" });
            continue;
          }
          if (!hasAppResourceFuncAnnotation(astNode, context.sourceCode)) {
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
