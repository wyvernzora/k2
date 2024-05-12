import debug = require("debug");

const LOG = debug("k2:release");

/**
 * Graph node that can be topologically sorted.
 */
export interface Node<T> {
  /**
   * Unique identifier for the node, used for identifying the node.
   */
  readonly id: string;
  /**
   * List of node's dependencies, represented by their IDs.
   */
  readonly deps: string[];
  /**
   * Value of the node.
   */
  readonly value: T;
}

export function toposort<T>(nodes: Array<Node<T>>): T[][] {
  const waves: T[][] = [];
  const visited: Set<string> = new Set();

  function isUnvisited(node: Node<T>): boolean {
    return !visited.has(node.id);
  }

  function depsSatisfied(node: Node<T>): boolean {
    const unsat = node.deps.filter((i) => !visited.has(i));
    LOG(`node ${node.id} unsatisfied deps`, unsat);
    return unsat.length === 0;
  }

  function visit(node: Node<T>): Node<T> {
    LOG(`visiting ${node.id}`);
    visited.add(node.id);
    return node;
  }

  function findNextWave() {
    return nodes.filter(isUnvisited).filter(depsSatisfied).map(visit);
  }

  while (visited.size < nodes.length) {
    const wave = findNextWave();
    if (wave.length === 0) {
      LOG(nodes.filter(isUnvisited).map((i) => i.id));
      throw new Error(`Circular or missing dependency`);
    }
    waves.push(wave.map((i) => i.value));
  }

  return waves;
}
