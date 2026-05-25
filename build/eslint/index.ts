import appIndexPublicApi from "./rules/app-index-public-api.js";
import componentLayout from "./rules/component-layout.js";
import noCdkLibAppImports from "./rules/no-cdk-lib-app-imports.js";
import noDeepInlineProps from "./rules/no-deep-inline-props.js";
import noLargeInlineConstructInstantiation from "./rules/no-large-inline-construct-instantiation.js";
import noRawApiObject from "./rules/no-raw-apiobject.js";
import preferCrdAliases from "./rules/prefer-crd-aliases.js";

export default {
  rules: {
    "app-index-public-api": appIndexPublicApi,
    "component-layout": componentLayout,
    "no-cdk-lib-app-imports": noCdkLibAppImports,
    "no-deep-inline-props": noDeepInlineProps,
    "no-large-inline-construct-instantiation": noLargeInlineConstructInstantiation,
    "no-raw-apiobject": noRawApiObject,
    "prefer-crd-aliases": preferCrdAliases,
  },
};
