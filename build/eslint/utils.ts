import fs from "node:fs";
import path from "node:path";

import type { Rule } from "eslint";

export interface AstNode {
  readonly type: string;
  readonly loc?: SourceLocation | null;
  readonly range?: [number, number];
  readonly parent?: AstNode | null;
  readonly [key: string]: unknown;
}

export interface SourceLocation {
  readonly start: { readonly line: number; readonly column: number };
  readonly end: { readonly line: number; readonly column: number };
}

export interface LiteralNode extends AstNode {
  readonly value?: unknown;
}

export interface ImportLikeNode extends AstNode {
  readonly source?: LiteralNode;
}

export interface ComponentPath {
  readonly appName: string;
  readonly componentName: string;
  readonly componentRoot: string;
  readonly rest: string[];
}

export function normalizedFilename(context: Rule.RuleContext): string {
  return normalizePath(context.physicalFilename === "<text>" ? context.filename : context.physicalFilename);
}

export function normalizePath(value: string): string {
  return value.replaceAll(path.sep, "/");
}

export function isGeneratedCrdFile(filename: string): boolean {
  return /(^|\/)apps\/[^/]+\/crds\/[^/]+\.ts$/.test(normalizePath(filename));
}

export function isAppIndexFile(filename: string): boolean {
  return /(^|\/)apps\/[^/]+\/index\.ts$/.test(normalizePath(filename));
}

export function isCdkLibFile(filename: string): boolean {
  return /(^|\/)cdk-lib\/.+\.ts$/.test(normalizePath(filename));
}

export function isAppOrCdkSourceFile(filename: string): boolean {
  const normalized = normalizePath(filename);
  return /(^|\/)(apps|cdk-lib)\//.test(normalized) && normalized.endsWith(".ts") && !isGeneratedCrdFile(normalized);
}

export function componentPath(filename: string): ComponentPath | undefined {
  const normalized = normalizePath(filename);
  const match = /(^|\/)apps\/([^/]+)\/components\/([^/]+)(?:\/(.*))?$/.exec(normalized);
  if (match === null) {
    return undefined;
  }
  const rest = match[4] === undefined ? [] : match[4].split("/");
  const prefix = normalized.slice(0, match.index + match[1].length);
  return {
    appName: match[2],
    componentName: match[3],
    componentRoot: `${prefix}/apps/${match[2]}/components/${match[3]}`,
    rest,
  };
}

export function sourceValue(node: ImportLikeNode): string | undefined {
  return typeof node.source?.value === "string" ? node.source.value : undefined;
}

export function exportedDeclarationNames(node: AstNode): string[] {
  const declaration = node.declaration as AstNode | undefined;
  if (declaration === undefined) {
    return [];
  }
  if (declaration.type === "VariableDeclaration") {
    const declarations = (declaration.declarations as AstNode[] | undefined) ?? [];
    return declarations.flatMap(item => identifierName(item.id as AstNode | undefined) ?? []);
  }
  const name = identifierName(declaration.id as AstNode | undefined);
  return name === undefined ? [] : [name];
}

export function identifierName(node: AstNode | undefined): string | undefined {
  return node?.type === "Identifier" && typeof node.name === "string" ? node.name : undefined;
}

export function hasAppResourceFuncAnnotation(node: AstNode, sourceCode: Rule.RuleContext["sourceCode"]): boolean {
  const declaration = node.declaration as AstNode | undefined;
  if (declaration?.type !== "VariableDeclaration") {
    return false;
  }
  const declarations = (declaration.declarations as AstNode[] | undefined) ?? [];
  if (declarations.length !== 1) {
    return false;
  }
  const id = declarations[0].id as AstNode | undefined;
  const typeAnnotation = id?.typeAnnotation as AstNode | undefined;
  return typeAnnotation !== undefined && getNodeText(sourceCode, typeAnnotation).includes("AppResourceFunc");
}

export function lineSpan(node: AstNode): number {
  if (node.loc === undefined || node.loc === null) {
    return 0;
  }
  return node.loc.end.line - node.loc.start.line + 1;
}

export function nonBlankLineCount(sourceCode: Rule.RuleContext["sourceCode"]): number {
  return sourceCode.lines.filter(line => line.trim().length > 0).length;
}

export function componentHasIndex(componentRoot: string): boolean {
  return fs.existsSync(`${componentRoot}/index.ts`);
}

export function isObjectExpression(node: unknown): node is AstNode {
  return isAstNode(node) && node.type === "ObjectExpression";
}

export function isAstNode(value: unknown): value is AstNode {
  return typeof value === "object" && value !== null && typeof (value as AstNode).type === "string";
}

export function objectDepth(node: AstNode): number {
  const properties = (node.properties as AstNode[] | undefined) ?? [];
  let maxChildDepth = 0;
  for (const property of properties) {
    const value = propertyValue(property);
    if (value === undefined) {
      continue;
    }
    maxChildDepth = Math.max(maxChildDepth, nestedObjectDepth(value));
  }
  return 1 + maxChildDepth;
}

export function memberExpressionText(node: AstNode, sourceCode: Rule.RuleContext["sourceCode"]): string | undefined {
  if (node.type !== "MemberExpression") {
    return undefined;
  }
  const text = getNodeText(sourceCode, node);
  return text.includes("\n") ? undefined : text;
}

export function isTopLevelMemberExpression(node: AstNode): boolean {
  const parent = node.parent;
  return !(parent?.type === "MemberExpression" && parent.object === node);
}

export function isAliasInitializer(node: AstNode): boolean {
  const parent = node.parent;
  if (parent?.type !== "VariableDeclarator" || parent.init !== node) {
    return false;
  }
  const name = identifierName(parent.id as AstNode | undefined);
  return name !== undefined && /^[A-Z]/.test(name);
}

function nestedObjectDepth(node: AstNode): number {
  if (node.type === "ObjectExpression") {
    return objectDepth(node);
  }
  if (node.type === "ArrayExpression") {
    const elements = (node.elements as Array<AstNode | null> | undefined) ?? [];
    return Math.max(0, ...elements.filter(isAstNode).map(nestedObjectDepth));
  }
  return 0;
}

function propertyValue(property: AstNode): AstNode | undefined {
  if (property.type === "Property" || property.type === "ObjectProperty") {
    return isAstNode(property.value) ? property.value : undefined;
  }
  if (property.type === "SpreadElement") {
    return isAstNode(property.argument) ? property.argument : undefined;
  }
  return undefined;
}

function getNodeText(sourceCode: Rule.RuleContext["sourceCode"], node: AstNode): string {
  return sourceCode.getText(node as unknown as Rule.Node);
}
