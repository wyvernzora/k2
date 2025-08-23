import tseslint from "typescript-eslint";
import stylistic from "@stylistic/eslint-plugin";
import prettier from "eslint-plugin-prettier";
import importPlugin from "eslint-plugin-import";

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
      import: importPlugin,
      prettier,
    },
    rules: {
      "prettier/prettier": "error",
      "import/order": [
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
    },
  },
);
