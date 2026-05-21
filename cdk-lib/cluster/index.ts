import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

import * as yaml from "js-yaml";

import type { ClusterConfig, ClusterConfigLoadOptions, ClusterConfigMap } from "./config.js";
import { CLUSTER_TARGETS } from "./target.js";
import type { ClusterTarget } from "./target.js";

export * from "./argo.js";
export * from "./config.js";
export * from "./context.js";
export * from "./deployment.js";
export * from "./kubernetes.js";
export * from "./one-password.js";
export * from "./target.js";

type JsonSchema = boolean | JsonSchemaObject;

interface JsonSchemaObject {
  readonly type?: string | readonly string[];
  readonly enum?: readonly unknown[];
  readonly required?: readonly string[];
  readonly properties?: Record<string, JsonSchema>;
  readonly additionalProperties?: boolean | JsonSchema;
  readonly items?: JsonSchema;
  readonly minItems?: number;
  readonly minLength?: number;
  readonly minimum?: number;
  readonly maximum?: number;
}

const DEFAULT_CLUSTERS_DIR = "clusters";
const DEFAULT_SCHEMA_PATH = fileURLToPath(new URL("cluster.schema.json", import.meta.url));

let cachedClusters: ClusterConfigMap | undefined;

export function loadClusters(options: ClusterConfigLoadOptions = {}): ClusterConfigMap {
  const clustersDir = path.resolve(options.clustersDir ?? DEFAULT_CLUSTERS_DIR);
  const schemaPath = options.schemaPath ? path.resolve(options.schemaPath) : DEFAULT_SCHEMA_PATH;
  const schema = loadSchema(schemaPath);

  const clusterConfigs = Object.fromEntries(
    CLUSTER_TARGETS.map(target => [target, loadClusterConfig(target, schema, clustersDir)]),
  ) as ClusterConfigMap;

  return clusterConfigs;
}

export function clusters(options: ClusterConfigLoadOptions = {}): ClusterConfigMap {
  if (options.clustersDir || options.schemaPath) {
    return loadClusters(options);
  }
  cachedClusters ??= loadClusters();
  return cachedClusters;
}

export function cluster(target: ClusterTarget): ClusterConfig {
  return clusters()[target];
}

export const CLUSTERS = clusters();

function loadClusterConfig(target: ClusterTarget, schema: JsonSchema, clustersDir: string): ClusterConfig {
  const file = path.join(clustersDir, `${target}.yaml`);
  const document = loadYamlFile(file);

  validateJsonSchema(document, schema, file);
  const config = document as ClusterConfig;
  if (config.id !== target) {
    throw new Error(`${file}: id must be ${JSON.stringify(target)}, got ${JSON.stringify(config.id)}`);
  }
  return config;
}

function loadSchema(schemaPath: string): JsonSchema {
  const schema = JSON.parse(fs.readFileSync(schemaPath, "utf8")) as unknown;
  if (!isJsonSchema(schema)) {
    throw new Error(`${schemaPath}: schema must be a JSON object or boolean`);
  }
  return schema;
}

function loadYamlFile(file: string): unknown {
  const document = yaml.load(fs.readFileSync(file, "utf8"));
  if (document == null) {
    throw new Error(`${file}: cluster config must not be empty`);
  }
  return document;
}

function validateJsonSchema(value: unknown, schema: JsonSchema, file: string): void {
  const errors: string[] = [];
  validateValue(value, schema, "$", errors);
  if (errors.length > 0) {
    throw new Error(
      `${file}: cluster config does not match schema:\n${errors.map(error => `  - ${error}`).join("\n")}`,
    );
  }
}

function validateValue(value: unknown, schema: JsonSchema, location: string, errors: string[]): void {
  if (schema === true) {
    return;
  }
  if (schema === false) {
    errors.push(`${location} is not allowed`);
    return;
  }

  if (schema.enum && !schema.enum.some(entry => valuesEqual(entry, value))) {
    errors.push(`${location} must be one of ${schema.enum.map(entry => JSON.stringify(entry)).join(", ")}`);
  }

  if (schema.type && !matchesType(value, schema.type)) {
    errors.push(`${location} must be ${formatType(schema.type)}, got ${formatValueType(value)}`);
    return;
  }

  if (typeof value === "string" && schema.minLength != null && value.length < schema.minLength) {
    errors.push(`${location} must be at least ${schema.minLength} characters`);
  }

  if (typeof value === "number") {
    if (schema.minimum != null && value < schema.minimum) {
      errors.push(`${location} must be >= ${schema.minimum}`);
    }
    if (schema.maximum != null && value > schema.maximum) {
      errors.push(`${location} must be <= ${schema.maximum}`);
    }
  }

  if (Array.isArray(value)) {
    if (schema.minItems != null && value.length < schema.minItems) {
      errors.push(`${location} must contain at least ${schema.minItems} item(s)`);
    }
    if (schema.items) {
      value.forEach((item, index) => validateValue(item, schema.items ?? true, `${location}[${index}]`, errors));
    }
  }

  if (isRecord(value)) {
    validateObject(value, schema, location, errors);
  }
}

function validateObject(
  value: Record<string, unknown>,
  schema: JsonSchemaObject,
  location: string,
  errors: string[],
): void {
  for (const key of schema.required ?? []) {
    if (!(key in value)) {
      errors.push(`${location}.${key} is required`);
    }
  }

  const properties = schema.properties ?? {};
  for (const [key, entry] of Object.entries(value)) {
    const propertySchema = properties[key];
    if (propertySchema) {
      validateValue(entry, propertySchema, `${location}.${key}`, errors);
      continue;
    }

    if (schema.additionalProperties === false) {
      errors.push(`${location}.${key} is not allowed`);
    } else if (schema.additionalProperties && typeof schema.additionalProperties === "object") {
      validateValue(entry, schema.additionalProperties, `${location}.${key}`, errors);
    }
  }
}

function matchesType(value: unknown, type: string | readonly string[]): boolean {
  const types = Array.isArray(type) ? type : [type];
  return types.some(entry => {
    switch (entry) {
      case "array":
        return Array.isArray(value);
      case "boolean":
        return typeof value === "boolean";
      case "integer":
        return Number.isInteger(value);
      case "number":
        return typeof value === "number";
      case "object":
        return isRecord(value);
      case "string":
        return typeof value === "string";
      default:
        return false;
    }
  });
}

function isJsonSchema(value: unknown): value is JsonSchema {
  return typeof value === "boolean" || isRecord(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value != null && !Array.isArray(value);
}

function valuesEqual(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

function formatType(type: string | readonly string[]): string {
  return typeof type === "string" ? type : type.join(" or ");
}

function formatValueType(value: unknown): string {
  if (Array.isArray(value)) {
    return "array";
  }
  if (value == null) {
    return "null";
  }
  if (Number.isInteger(value)) {
    return "integer";
  }
  return typeof value;
}
