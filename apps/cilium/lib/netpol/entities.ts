import {
  CiliumClusterwideNetworkPolicySpecsEgressToEntities,
  CiliumClusterwideNetworkPolicySpecsIngressFromEntities,
} from "../../crds/cilium.io.js";

import type { EntityPeer } from "./types.js";

const IngressEntity = CiliumClusterwideNetworkPolicySpecsIngressFromEntities;
const EgressEntity = CiliumClusterwideNetworkPolicySpecsEgressToEntities;

export function clusterwideIngressEntity(entity: EntityPeer): CiliumClusterwideNetworkPolicySpecsIngressFromEntities {
  switch (entity) {
    case "world":
      return IngressEntity.WORLD;
    case "cluster":
      return IngressEntity.CLUSTER;
    case "host":
      return IngressEntity.HOST;
    case "remote-node":
      return IngressEntity.REMOTE_HYPHEN_NODE;
    case "kube-apiserver":
      return IngressEntity.KUBE_HYPHEN_APISERVER;
    case "cilium-ingress":
      return IngressEntity.INGRESS;
  }
}

export function clusterwideEgressEntity(entity: EntityPeer): CiliumClusterwideNetworkPolicySpecsEgressToEntities {
  switch (entity) {
    case "world":
      return EgressEntity.WORLD;
    case "cluster":
      return EgressEntity.CLUSTER;
    case "host":
      return EgressEntity.HOST;
    case "remote-node":
      return EgressEntity.REMOTE_HYPHEN_NODE;
    case "kube-apiserver":
      return EgressEntity.KUBE_HYPHEN_APISERVER;
    case "cilium-ingress":
      return EgressEntity.INGRESS;
  }
}
