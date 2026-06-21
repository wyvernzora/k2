import appIndexPublicApi from "./rules/app-index-public-api.js";
import componentLayout from "./rules/component-layout.js";
import noCdkLibAppImports from "./rules/no-cdk-lib-app-imports.js";
import noCdk8sPlusDeepImports from "./rules/no-cdk8s-plus-deep-imports.js";
import noDeepInlineProps from "./rules/no-deep-inline-props.js";
import noLargeInlineConstructInstantiation from "./rules/no-large-inline-construct-instantiation.js";
import noRawApiObject from "./rules/no-raw-apiobject.js";
import noRawK8sJobs from "./rules/no-raw-k8s-jobs.js";
import noSingleUseConstantsModule from "./rules/no-single-use-constants-module.js";
import preferCdk8sPlusL2 from "./rules/prefer-cdk8s-plus-l2.js";
import preferCrdAliases from "./rules/prefer-crd-aliases.js";

export default {
  rules: {
    "app-index-public-api": appIndexPublicApi,
    "component-layout": componentLayout,
    "no-cdk-lib-app-imports": noCdkLibAppImports,
    "no-cdk8s-plus-deep-imports": noCdk8sPlusDeepImports,
    "no-deep-inline-props": noDeepInlineProps,
    "no-large-inline-construct-instantiation": noLargeInlineConstructInstantiation,
    "no-raw-apiobject": noRawApiObject,
    "no-raw-k8s-jobs": noRawK8sJobs,
    "no-single-use-constants-module": noSingleUseConstantsModule,
    "prefer-cdk8s-plus-l2": preferCdk8sPlusL2,
    "prefer-crd-aliases": preferCrdAliases,
  },
};
