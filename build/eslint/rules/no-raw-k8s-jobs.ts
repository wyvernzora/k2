import type { Rule } from "eslint";

import {
  identifierName,
  isAppOrCdkSourceFile,
  normalizedFilename,
  sourceValue,
  type AstNode,
  type ImportLikeNode,
} from "../utils.js";

const RULE_NAME = "no-raw-k8s-jobs";
const K2_RULE_NAME = `k2/${RULE_NAME}`;
const RAW_JOB_IMPORTS = new Set(["CronJob", "Job"]);

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: {
      description: "Require K2 job wrappers instead of raw cdk8s-plus Job and CronJob constructs.",
    },
    messages: {
      rawJob:
        "Use K2 ScriptedJob or ScriptedCronJob instead of raw cdk8s-plus Job/CronJob; disable with a reason only when intentional.",
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
        const astNode = node as unknown as AstNode;
        const source = sourceValue(astNode as ImportLikeNode);
        if (source !== "cdk8s-plus-32") {
          return;
        }

        for (const specifier of importSpecifiers(astNode)) {
          if (isRawJobImport(specifier, astNode)) {
            context.report({ node: specifier as unknown as Rule.Node, messageId: "rawJob" });
          }
        }
      },
    };
  },
};

export default rule;

function importSpecifiers(node: AstNode): AstNode[] {
  return (node.specifiers as AstNode[] | undefined) ?? [];
}

function isRawJobImport(specifier: AstNode, declaration: AstNode): boolean {
  if (specifier.type !== "ImportSpecifier" || isTypeOnlyImport(specifier, declaration)) {
    return false;
  }
  const name = identifierName(specifier.imported as AstNode | undefined);
  return name !== undefined && RAW_JOB_IMPORTS.has(name);
}

function isTypeOnlyImport(specifier: AstNode, declaration: AstNode): boolean {
  return declaration.importKind === "type" || specifier.importKind === "type";
}

function isTargetDisableComment(value: string): boolean {
  return /\beslint-disable(?:-next-line|-line)?\b/.test(value) && value.includes(`/${RULE_NAME}`);
}

function hasDisableReason(value: string): boolean {
  return /--\s+\S/.test(value);
}
