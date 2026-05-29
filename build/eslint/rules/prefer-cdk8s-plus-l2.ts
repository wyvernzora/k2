import type { Rule } from "eslint";

import {
  identifierName,
  isAppOrCdkSourceFile,
  normalizedFilename,
  sourceValue,
  type AstNode,
  type ImportLikeNode,
} from "../utils.js";

const RULE_NAME = "prefer-cdk8s-plus-l2";
const K2_RULE_NAME = `k2/${RULE_NAME}`;

const rule: Rule.RuleModule = {
  meta: {
    type: "suggestion",
    docs: {
      description: "Discourage raw Kubernetes L1 constructs when cdk8s-plus L2 constructs are available.",
    },
    messages: {
      kubeConstruct:
        "Prefer top-level cdk8s-plus L2 constructs over raw Kube* constructs; disable with a reason only when L1 is unavoidable.",
      namespaceUse:
        "Prefer top-level cdk8s-plus L2 constructs over k8s.* raw Kubernetes bindings; disable with a reason only when L1 is unavoidable.",
      missingDisableReason: `Disabling ${K2_RULE_NAME} requires a "-- reason" description.`,
    },
    schema: [
      {
        type: "object",
        properties: {
          allowedTypes: {
            type: "array",
            items: { type: "string" },
            uniqueItems: true,
          },
        },
        additionalProperties: false,
      },
    ],
  },
  create(context) {
    if (!isAppOrCdkSourceFile(normalizedFilename(context))) {
      return {};
    }

    const allowedTypes = allowedTypeNames(context.options);
    const k8sNamespaceImports = new Set<string>();
    const rawKubeImports = new Map<string, string>();

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
        if (source === undefined || !isCdk8sPlusImport(source)) {
          return;
        }

        for (const specifier of importSpecifiers(astNode)) {
          if (isK8sNamespaceImport(specifier)) {
            k8sNamespaceImports.add(localName(specifier));
          }
          if (isRawKubeImport(source, specifier)) {
            rawKubeImports.set(localName(specifier), importedName(specifier) ?? "");
          }
        }
      },
      MemberExpression(node) {
        const astNode = node as unknown as AstNode;
        const objectName = identifierName(astNode.object as AstNode | undefined);
        const propertyName = identifierName(astNode.property as AstNode | undefined);
        if (objectName !== undefined && k8sNamespaceImports.has(objectName) && !isAllowed(propertyName, allowedTypes)) {
          context.report({ node, messageId: "namespaceUse" });
        }
      },
      TSQualifiedName(node: unknown) {
        const astNode = node as unknown as AstNode;
        const leftName = identifierName(astNode.left as AstNode | undefined);
        const rightName = identifierName(astNode.right as AstNode | undefined);
        if (leftName !== undefined && k8sNamespaceImports.has(leftName) && !isAllowed(rightName, allowedTypes)) {
          context.report({ node: node as Rule.Node, messageId: "namespaceUse" });
        }
      },
      NewExpression(node) {
        const callee = (node as unknown as AstNode).callee as AstNode | undefined;
        if (isRawKubeIdentifier(callee, rawKubeImports, allowedTypes)) {
          context.report({ node, messageId: "kubeConstruct" });
        }
      },
      ClassDeclaration(node) {
        const superClass = (node as unknown as AstNode).superClass as AstNode | undefined;
        if (isRawKubeIdentifier(superClass, rawKubeImports, allowedTypes)) {
          context.report({ node, messageId: "kubeConstruct" });
        }
      },
      ClassExpression(node) {
        const superClass = (node as unknown as AstNode).superClass as AstNode | undefined;
        if (isRawKubeIdentifier(superClass, rawKubeImports, allowedTypes)) {
          context.report({ node, messageId: "kubeConstruct" });
        }
      },
    };
  },
};

export default rule;

function isCdk8sPlusImport(source: string): boolean {
  return /^cdk8s-plus-[^/]+(?:\/.*)?$/.test(source);
}

function isCdk8sPlusRawImport(source: string): boolean {
  return /^cdk8s-plus-[^/]+\/lib\/imports\/k8s(?:\.js)?$/.test(source);
}

function importSpecifiers(node: AstNode): AstNode[] {
  return (node.specifiers as AstNode[] | undefined) ?? [];
}

function isK8sNamespaceImport(specifier: AstNode): boolean {
  if (specifier.type === "ImportSpecifier") {
    return importedName(specifier) === "k8s";
  }
  return false;
}

function isRawKubeImport(source: string, specifier: AstNode): boolean {
  if (!isCdk8sPlusRawImport(source) || specifier.type !== "ImportSpecifier") {
    return false;
  }
  const name = importedName(specifier);
  return name !== undefined && /^Kube[A-Z]/.test(name);
}

function importedName(specifier: AstNode): string | undefined {
  return identifierName(specifier.imported as AstNode | undefined);
}

function localName(specifier: AstNode): string {
  return identifierName(specifier.local as AstNode | undefined) ?? "";
}

function isRawKubeIdentifier(
  node: AstNode | undefined,
  rawKubeImports: Map<string, string>,
  allowedTypes: Set<string>,
): boolean {
  const name = identifierName(node);
  const importedName = name === undefined ? undefined : rawKubeImports.get(name);
  return importedName !== undefined && !allowedTypes.has(importedName);
}

function isAllowed(typeName: string | undefined, allowedTypes: Set<string>): boolean {
  return typeName !== undefined && allowedTypes.has(typeName);
}

function allowedTypeNames(options: unknown[]): Set<string> {
  const [rawOptions] = options as Array<{ readonly allowedTypes?: unknown } | undefined>;
  const allowedTypes = rawOptions?.allowedTypes;
  if (!Array.isArray(allowedTypes)) {
    return new Set();
  }
  return new Set(allowedTypes.filter((item): item is string => typeof item === "string"));
}

function isTargetDisableComment(value: string): boolean {
  return /\beslint-disable(?:-next-line|-line)?\b/.test(value) && value.includes(`/${RULE_NAME}`);
}

function hasDisableReason(value: string): boolean {
  return /--\s+\S/.test(value);
}
