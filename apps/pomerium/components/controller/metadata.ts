import { POMERIUM_NAMESPACE } from "../../lib/constants.js";

const APP_NAME = "pomerium";
const DEFAULT_COMPONENT = "pomerium";

export function metadata(name: string, component = DEFAULT_COMPONENT) {
  return {
    name,
    namespace: POMERIUM_NAMESPACE,
    labels: componentLabels(component),
  };
}

export function clusterMetadata(name: string, component = DEFAULT_COMPONENT) {
  return {
    name,
    labels: componentLabels(component),
  };
}

export function componentLabels(component = DEFAULT_COMPONENT) {
  return {
    "k2.wyvernzora.io/app": APP_NAME,
    "k2.wyvernzora.io/component": component,
  };
}
