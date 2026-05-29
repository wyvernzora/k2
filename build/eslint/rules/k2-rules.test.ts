import path from "node:path";
import process from "node:process";

import { RuleTester } from "eslint";
import tseslint from "typescript-eslint";

import appIndexPublicApi from "./app-index-public-api.js";
import componentLayout from "./component-layout.js";
import noCdkLibAppImports from "./no-cdk-lib-app-imports.js";
import noCdk8sPlusDeepImports from "./no-cdk8s-plus-deep-imports.js";
import noDeepInlineProps from "./no-deep-inline-props.js";
import noLargeInlineConstructInstantiation from "./no-large-inline-construct-instantiation.js";
import noRawApiObject from "./no-raw-apiobject.js";
import preferCdk8sPlusL2 from "./prefer-cdk8s-plus-l2.js";
import preferCrdAliases from "./prefer-crd-aliases.js";

const ruleTesterHooks = RuleTester as unknown as {
  describe: (name: string, fn: () => void) => void;
  it: (name: string, fn: () => void) => void;
  itOnly: (name: string, fn: () => void) => void;
};
const suiteNames: string[] = [];
ruleTesterHooks.describe = (name, fn) => {
  suiteNames.push(name);
  try {
    fn();
  } finally {
    suiteNames.pop();
  }
};
ruleTesterHooks.it = (name, fn) => {
  try {
    fn();
  } catch (error) {
    if (error instanceof Error) {
      error.message = `${[...suiteNames, name].join(" > ")}: ${error.message}`;
    }
    throw error;
  }
};
ruleTesterHooks.itOnly = ruleTesterHooks.it;

const tester = new RuleTester({
  languageOptions: {
    ecmaVersion: 2024,
    parser: tseslint.parser,
    parserOptions: {
      sourceType: "module",
    },
  },
});
const repoRoot = process.cwd();
const repoFile = (...parts: string[]) => path.join(repoRoot, ...parts);

tester.run("app-index-public-api", appIndexPublicApi, {
  valid: [
    {
      filename: repoFile("apps/demo/index.ts"),
      code: `
        import type { AppResourceFunc } from "@k2/cdk-lib";
        export * as crd from "./lib/crd.js";
        export { Thing, type ThingProps } from "./lib/thing.js";
        export const createAppResources: AppResourceFunc = app => app;
      `,
    },
  ],
  invalid: [
    {
      filename: repoFile("apps/demo/index.ts"),
      code: `export * from "./components/demo/index.js";`,
      errors: [{ messageId: "nonLibReExport" }],
    },
    {
      filename: repoFile("apps/demo/index.ts"),
      code: `export const createArgoCdApp = () => undefined;`,
      errors: [{ messageId: "unexpectedExport" }],
    },
    {
      filename: repoFile("apps/demo/index.ts"),
      code: `export const createAppResources = app => app;`,
      errors: [{ messageId: "missingType" }],
    },
  ],
});

tester.run("component-layout", componentLayout, {
  valid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `export class Demo {}`,
    },
    {
      filename: repoFile("apps/cert-manager/components/cert-manager/certificate.ts"),
      code: `export class Certificate {}`,
    },
  ],
  invalid: [
    {
      filename: repoFile("apps/demo/components/huge.ts"),
      code: Array.from({ length: 101 }, (_, index) => `const line${index} = ${index};`).join("\n"),
      errors: [{ messageId: "tooLarge" }],
    },
    {
      filename: repoFile("apps/demo/components/missing/deployment.ts"),
      code: `export class Deployment {}`,
      errors: [{ messageId: "missingIndex" }],
    },
  ],
});

tester.run("no-large-inline-construct-instantiation", noLargeInlineConstructInstantiation, {
  valid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `new Thing(this, "thing", buildThingProps());`,
    },
  ],
  invalid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        new Thing(
          this,
          "thing",
          {
${Array.from({ length: 31 }, (_, index) => `            key${index}: ${index},`).join("\n")}
          },
        );
      `,
      errors: [{ messageId: "tooLarge" }],
    },
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        new Thing(
          this,
          "thing",
${Array.from({ length: 31 }, (_, index) => `          buildArg${index}(),`).join("\n")}
        );
      `,
      errors: [{ messageId: "tooLarge" }],
    },
  ],
});

tester.run("no-deep-inline-props", noDeepInlineProps, {
  valid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `super(scope, id, buildProps());`,
    },
  ],
  invalid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        const config = {
          spec: {
            template: {
              spec: {
                containers: [],
              },
            },
          },
        };
      `,
      errors: [{ messageId: "tooDeep" }],
    },
  ],
});

tester.run("no-raw-apiobject", noRawApiObject, {
  valid: [
    {
      filename: repoFile("build/eslint/rules/test.ts"),
      code: `new ApiObject(this, "x", {});`,
    },
  ],
  invalid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `new ApiObject(this, "x", {});`,
      errors: [{ messageId: "rawApiObject" }],
    },
  ],
});

tester.run("no-cdk-lib-app-imports", noCdkLibAppImports, {
  valid: [
    {
      filename: repoFile("cdk-lib/context/namespace.ts"),
      code: `import { Construct } from "constructs";`,
    },
  ],
  invalid: [
    {
      filename: repoFile("cdk-lib/context/namespace.ts"),
      code: `import { crd } from "@k2/external-secrets";`,
      errors: [{ messageId: "appImport" }],
    },
    {
      filename: repoFile("cdk-lib/context/namespace.ts"),
      code: `import { Thing } from "../apps/demo/lib/thing.js";`,
      errors: [{ messageId: "appImport" }],
    },
  ],
});

tester.run("no-cdk8s-plus-deep-imports", noCdk8sPlusDeepImports, {
  valid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `import { k8s, Service } from "cdk8s-plus-32";`,
    },
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        // eslint-disable-next-line rule-to-test/no-cdk8s-plus-deep-imports -- Required while validating upstream import behavior.
        import { KubeService } from "cdk8s-plus-32/lib/imports/k8s.js";
      `,
    },
  ],
  invalid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `import { KubeService } from "cdk8s-plus-32/lib/imports/k8s.js";`,
      errors: [{ messageId: "deepImport" }],
    },
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        // eslint-disable-next-line rule-to-test/no-cdk8s-plus-deep-imports
        const value = 1;
      `,
      errors: [{ messageId: "missingDisableReason" }],
    },
  ],
});

tester.run("prefer-cdk8s-plus-l2", preferCdk8sPlusL2, {
  valid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `import { Deployment, Service } from "cdk8s-plus-32";`,
    },
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        import { k8s } from "cdk8s-plus-32";
        // eslint-disable-next-line rule-to-test/prefer-cdk8s-plus-l2 -- Job TTL is not exposed by the L2 construct.
        const job = new k8s.KubeJob(this, "job", {});
      `,
    },
    {
      filename: repoFile("cdk-lib/scheduling.ts"),
      code: `
        import type { k8s } from "cdk8s-plus-32";
        type Profile = { affinity?: k8s.Affinity; tolerations?: k8s.Toleration[] };
      `,
      options: [{ allowedTypes: ["Affinity", "Toleration"] }],
    },
  ],
  invalid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        import { k8s } from "cdk8s-plus-32";
        new k8s.KubeService(this, "service", {});
      `,
      errors: [{ messageId: "namespaceUse" }],
    },
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        import { k8s } from "cdk8s-plus-32";
        type Container = k8s.Container;
      `,
      errors: [{ messageId: "namespaceUse" }],
    },
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        // eslint-disable-next-line rule-to-test/prefer-cdk8s-plus-l2
        const value = 1;
      `,
      errors: [{ messageId: "missingDisableReason" }],
    },
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        import { KubeService } from "cdk8s-plus-32/lib/imports/k8s.js";
        new KubeService(this, "service", {});
      `,
      errors: [{ messageId: "kubeConstruct" }],
    },
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        import { KubeDeployment } from "cdk8s-plus-32/lib/imports/k8s.js";
        class DemoDeployment extends KubeDeployment {}
      `,
      errors: [{ messageId: "kubeConstruct" }],
    },
  ],
});

tester.run("prefer-crd-aliases", preferCrdAliases, {
  valid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        const { ClusterExternalSecretSpecExternalSecretSpecTargetCreationPolicy: TargetCreationPolicy } = crd;
        const owner = TargetCreationPolicy.OWNER;
        const merge = TargetCreationPolicy.MERGE;
      `,
    },
  ],
  invalid: [
    {
      filename: repoFile("apps/demo/components/demo.ts"),
      code: `
        const owner = crd.ClusterExternalSecretSpecExternalSecretSpecTargetCreationPolicy.OWNER;
      `,
      errors: [{ messageId: "preferAlias" }],
    },
  ],
});
