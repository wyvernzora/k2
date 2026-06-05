import tseslint from "typescript-eslint";
import stylistic from "@stylistic/eslint-plugin";
import prettier from "eslint-plugin-prettier";
import importPlugin from "eslint-plugin-import-x";

import k2 from "./build/eslint/index.js";

export default tseslint.config(
  // Global ignores
  {
    ignores: [
      "**/dist/**",
      "node_modules",
      "**/crds/*.ts", // skip auto-generated CRD bindings
    ],
  },
  // Recommended base configs
  ...tseslint.configs.recommended,

  // Project rules
  {
    files: ["**/*.ts", "**/*.tsx"],
    plugins: {
      "@stylistic": stylistic,
      "import-x": importPlugin,
      k2,
      prettier,
    },
    rules: {
      "prettier/prettier": "error",
      "import-x/order": [
        "error",
        {
          groups: ["builtin", "external", "internal", "parent", "sibling", "index"],
          "newlines-between": "always",
          pathGroups: [
            {
              pattern: "@k2/**",
              group: "internal",
              position: "after",
            },
          ],
          pathGroupsExcludedImportTypes: ["builtin"],
        },
      ],
      "@stylistic/quote-props": ["error", "as-needed"],
      "k2/app-index-public-api": "error",
      "k2/component-layout": "error",
      "k2/no-cdk-lib-app-imports": "error",
      "k2/no-cdk8s-plus-deep-imports": "error",
      "k2/no-deep-inline-props": "error",
      "k2/no-large-inline-construct-instantiation": "error",
      "k2/no-raw-apiobject": "error",
      "k2/no-single-use-constants-module": [
        "error",
        {
          allowedModules: [
            "apps/cert-manager/components/cert-manager/constants.ts",
            "apps/pocket-id/constants.ts",
            "apps/pomerium/constants.ts",
          ],
        },
      ],
      "k2/prefer-cdk8s-plus-l2": [
        "warn",
        {
          allowedTypes: ["Affinity", "Toleration"],
        },
      ],
      "k2/prefer-crd-aliases": "warn",
    },
  },
);
