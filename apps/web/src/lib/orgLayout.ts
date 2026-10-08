// The Org chart's tree layout, Paperclip's (ui/src/pages/OrgChart.tsx; MIT,
// see NOTICE): each subtree is as wide as its reports side by side, a
// Manager sits centred above them, and the roots stand in a row.
import type { AgentStatus, OrgNode } from "./agents";

export const CARD_W = 200;
export const CARD_H = 100;
export const GAP_X = 32;
export const GAP_Y = 80;
export const PADDING = 60;
export const MIN_ZOOM = 0.2;
export const MAX_ZOOM = 2;
export const FIT_PADDING = 40;

export type Point = { x: number; y: number };

/** An OrgNode with its card's top-left corner. */
export type LayoutNode = Omit<OrgNode, "reports"> & {
  x: number;
  y: number;
  children: LayoutNode[];
};

/** The width a subtree needs. */
export function subtreeWidth(node: OrgNode): number {
  if (node.reports.length === 0) return CARD_W;
  const childrenW = node.reports.reduce((sum, c) => sum + subtreeWidth(c), 0);
  return Math.max(CARD_W, childrenW + (node.reports.length - 1) * GAP_X);
}

function layoutTree(node: OrgNode, x: number, y: number): LayoutNode {
  const { reports, ...card } = node;
  const totalW = subtreeWidth(node);
  const children: LayoutNode[] = [];
  if (reports.length > 0) {
    const childrenW = reports.reduce((sum, c) => sum + subtreeWidth(c), 0);
    let cx = x + (totalW - childrenW - (reports.length - 1) * GAP_X) / 2;
    for (const child of reports) {
      children.push(layoutTree(child, cx, y + CARD_H + GAP_Y));
      cx += subtreeWidth(child) + GAP_X;
    }
  }
  return { ...card, x: x + (totalW - CARD_W) / 2, y, children };
}

/** Lays the roots out side by side from the padding corner. */
export function layoutForest(roots: OrgNode[]): LayoutNode[] {
  let x = PADDING;
  return roots.map((root) => {
    const laid = layoutTree(root, x, PADDING);
    x += subtreeWidth(root) + GAP_X;
    return laid;
  });
}

/** Every card, Managers before their reports. */
export function flattenLayout(nodes: LayoutNode[]): LayoutNode[] {
  return nodes.flatMap((n) => [n, ...flattenLayout(n.children)]);
}

/** Every Manager-to-report line. */
export function collectEdges(
  nodes: LayoutNode[],
): { parent: LayoutNode; child: LayoutNode }[] {
  return nodes.flatMap((n) => [
    ...n.children.map((child) => ({ parent: n, child })),
    ...collectEdges(n.children),
  ]);
}

/** The chart's size: the far card corner plus the padding. */
export function chartBounds(nodes: LayoutNode[]): { width: number; height: number } {
  if (nodes.length === 0) return { width: 800, height: 600 };
  const maxX = Math.max(...nodes.map((n) => n.x + CARD_W));
  const maxY = Math.max(...nodes.map((n) => n.y + CARD_H));
  return { width: maxX + PADDING, height: maxY + PADDING };
}

export const clampZoom = (value: number) =>
  Math.min(Math.max(value, MIN_ZOOM), MAX_ZOOM);

/** The zoom and pan that centre the whole chart, never above 100%; null
 * when the viewport is too small to fit anything. */
export function fitChartToViewport(
  width: number,
  height: number,
  bounds: { width: number; height: number },
): { zoom: number; pan: Point } | null {
  if (width <= FIT_PADDING || height <= FIT_PADDING) return null;
  const zoom = clampZoom(
    Math.min((width - FIT_PADDING) / bounds.width, (height - FIT_PADDING) / bounds.height, 1),
  );
  return {
    zoom,
    pan: { x: (width - bounds.width * zoom) / 2, y: (height - bounds.height * zoom) / 2 },
  };
}

/** The pan that keeps `point` still while the zoom changes. */
export const panToward = (point: Point, pan: Point, from: number, to: number): Point => ({
  x: point.x - (to / from) * (point.x - pan.x),
  y: point.y - (to / from) * (point.y - pan.y),
});

/** Paperclip's filterOrgTree: a node stays when it matches the tab's
 * statuses or any of its reports does. */
export function filterOrgTree(
  nodes: OrgNode[],
  keep: readonly AgentStatus[],
): OrgNode[] {
  return nodes.flatMap((node) => {
    const reports = filterOrgTree(node.reports, keep);
    return keep.includes(node.status) || reports.length > 0
      ? [{ ...node, reports }]
      : [];
  });
}
