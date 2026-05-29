import fs from "node:fs";
import path from "node:path";
import process from "node:process";

import type { Rule } from "eslint";

import {
  identifierName,
  isAppOrCdkSourceFile,
  normalizePath,
  normalizedFilename,
  sourceValue,
  type AstNode,
  type ImportLikeNode,
} from "../utils.js";

const MIN_IMPORTERS = 2;
const MODULE_IMPORT = "*";
const SKIPPED_DIRECTORIES = new Set([".git", ".main", ".checkpoint", "deploy", "dist", "node_modules"]);

const importerCountCache = new Map<string, number>();

const rule: Rule.RuleModule = {
  meta: {
    type: "problem",
    docs: {
      description: "Prevent app-local constants modules from being used for values with only one consumer.",
    },
    messages: {
      singleUseConstants:
        "Inline single-use constant {{name}} in its consumer. Keep shared constants only once each exported value has multiple real importers.",
    },
    schema: [
      {
        type: "object",
        properties: {
          allowedModules: {
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
    const filename = normalizedFilename(context);
    if (!isAppOrCdkSourceFile(filename)) {
      return {};
    }

    return {
      ImportDeclaration(node) {
        const astNode = node as unknown as AstNode;
        const source = sourceValue(astNode as ImportLikeNode);
        const target = source === undefined ? undefined : resolveConstantsImport(filename, source);
        if (target === undefined) {
          return;
        }

        const root = sourceRoot(filename);
        if (allowedModules(context.options).has(relativePath(root, target))) {
          return;
        }

        for (const name of importedNames(astNode)) {
          if (importerCount(target, name, root) < MIN_IMPORTERS) {
            context.report({ node, messageId: "singleUseConstants", data: { name } });
          }
        }
      },
    };
  },
};

export default rule;

function resolveConstantsImport(importerFilename: string, source: string): string | undefined {
  if (!source.startsWith(".")) {
    return undefined;
  }

  const target = normalizePath(path.resolve(path.dirname(importerFilename), source.replace(/\.js$/, ".ts")));
  if (path.basename(target) !== "constants.ts" || !fs.existsSync(target)) {
    return undefined;
  }

  return target;
}

function importerCount(target: string, name: string, root: string): number {
  const cacheKey = `${root}:${target}:${name}`;
  const cached = importerCountCache.get(cacheKey);
  if (cached !== undefined) {
    return cached;
  }

  const count = tsFiles(root).filter(file => importNamesForTarget(file, target).has(name)).length;
  importerCountCache.set(cacheKey, count);
  return count;
}

function sourceRoot(filename: string): string {
  const normalized = normalizePath(filename);
  const match = /^(.*)\/(?:apps|build|cdk-lib|tools)\//.exec(normalized);
  return match?.[1] ?? normalizePath(process.cwd());
}

function relativePath(root: string, filename: string): string {
  return normalizePath(path.relative(root, filename));
}

function allowedModules(options: unknown[]): Set<string> {
  const [rawOptions] = options as Array<{ readonly allowedModules?: unknown } | undefined>;
  const modules = rawOptions?.allowedModules;
  if (!Array.isArray(modules)) {
    return new Set();
  }
  return new Set(modules.filter((item): item is string => typeof item === "string"));
}

function tsFiles(root: string): string[] {
  if (!fs.existsSync(root)) {
    return [];
  }
  const entries = fs.readdirSync(root, { withFileTypes: true });
  return entries.flatMap(entry => {
    if (SKIPPED_DIRECTORIES.has(entry.name)) {
      return [];
    }

    const entryPath = path.join(root, entry.name);
    if (entry.isDirectory()) {
      return tsFiles(entryPath);
    }
    if (!entry.isFile() || !entry.name.endsWith(".ts")) {
      return [];
    }
    return [normalizePath(entryPath)];
  });
}

function importNamesForTarget(filename: string, target: string): Set<string> {
  const content = fs.readFileSync(filename, "utf8");
  const names = new Set<string>();
  for (const importDeclaration of importDeclarations(content)) {
    if (resolveConstantsImport(filename, importDeclaration.source) !== target) {
      continue;
    }
    for (const name of importedNamesFromBindings(importDeclaration.bindings)) {
      names.add(name);
    }
  }
  return names;
}

function importedNames(node: AstNode): string[] {
  const specifiers = (node.specifiers as AstNode[] | undefined) ?? [];
  const names = specifiers.flatMap(specifierImportedName);
  return names.length === 0 ? [MODULE_IMPORT] : names;
}

function specifierImportedName(specifier: AstNode): string[] {
  if (specifier.type !== "ImportSpecifier") {
    return [MODULE_IMPORT];
  }
  const name = identifierName(specifier.imported as AstNode | undefined);
  return name === undefined ? [] : [name];
}

interface ImportDeclarationText {
  readonly bindings: string;
  readonly source: string;
}

function importDeclarations(content: string): ImportDeclarationText[] {
  return Array.from(content.matchAll(/\bimport\s+(?:type\s+)?([\s\S]*?)\s+from\s+["']([^"']+)["']/g), match => ({
    bindings: match[1],
    source: match[2],
  }));
}

function importedNamesFromBindings(bindings: string): string[] {
  const namedImport = /\{([\s\S]*?)\}/.exec(bindings);
  if (namedImport === null) {
    return [MODULE_IMPORT];
  }
  return namedImport[1]
    .split(",")
    .map(
      item =>
        item
          .trim()
          .replace(/^type\s+/, "")
          .split(/\s+as\s+/)[0]
          ?.trim() ?? "",
    )
    .filter(item => item.length > 0);
}
