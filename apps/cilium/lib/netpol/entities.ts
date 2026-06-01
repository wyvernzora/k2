import {
  CiliumClusterwideNetworkPolicySpecsEgressToEntities,
  CiliumClusterwideNetworkPolicySpecsIngressDenyFromEntities,
  CiliumClusterwideNetworkPolicySpecsIngressFromEntities,
} from "../../crds/cilium.io.js";

import type { EntityPeer } from "./types.js";

const IngressEntity = CiliumClusterwideNetworkPolicySpecsIngressFromEntities;
const IngressDenyEntity = CiliumClusterwideNetworkPolicySpecsIngressDenyFromEntities;
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

export function clusterwideIngressDenyEntity(
  entity: EntityPeer,
): CiliumClusterwideNetworkPolicySpecsIngressDenyFromEntities {
  switch (entity) {
    case "world":
      return IngressDenyEntity.WORLD;
    case "cluster":
      return IngressDenyEntity.CLUSTER;
    case "host":
      return IngressDenyEntity.HOST;
    case "remote-node":
      return IngressDenyEntity.REMOTE_HYPHEN_NODE;
    case "kube-apiserver":
      return IngressDenyEntity.KUBE_HYPHEN_APISERVER;
    case "cilium-ingress":
      return IngressDenyEntity.INGRESS;
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
