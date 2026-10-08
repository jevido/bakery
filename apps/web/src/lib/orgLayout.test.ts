import { expect, test } from "bun:test";
import type { AgentStatus, OrgNode } from "./agents";
import {
  CARD_H,
  CARD_W,
  GAP_X,
  GAP_Y,
  PADDING,
  chartBounds,
  collectEdges,
  filterOrgTree,
  fitChartToViewport,
  flattenLayout,
  layoutForest,
  subtreeWidth,
} from "./orgLayout";

const node = (id: number, reports: OrgNode[] = [], status: AgentStatus = "idle"): OrgNode => ({
  id, name: `a${id}`, job: "engineer", job_label: "Engineer", title: "", icon: "bot", status, reports,
});

test("two roots stand side by side", () => {
  const [a, b] = layoutForest([node(1), node(2)]);
  expect([a.x, a.y]).toEqual([PADDING, PADDING]);
  expect([b.x, b.y]).toEqual([PADDING + CARD_W + GAP_X, PADDING]);
});

test("a three-level chain is one column, a level per row", () => {
  const laid = flattenLayout(layoutForest([node(1, [node(2, [node(3)])])]));
  expect(laid.map((n) => [n.id, n.x, n.y])).toEqual([
    [1, PADDING, PADDING],
    [2, PADDING, PADDING + CARD_H + GAP_Y],
    [3, PADDING, PADDING + 2 * (CARD_H + GAP_Y)],
  ]);
  expect(collectEdges(layoutForest([node(1, [node(2, [node(3)])])])).map((e) => [e.parent.id, e.child.id])).toEqual([[1, 2], [2, 3]]);
});

test("a Manager is as wide as its reports and centred above them", () => {
  const tree = node(1, [node(2), node(3), node(4)]);
  expect(subtreeWidth(tree)).toBe(3 * CARD_W + 2 * GAP_X);
  const [root] = layoutForest([tree]);
  expect(root.x).toBe(PADDING + CARD_W + GAP_X);
  expect(root.children.map((c) => c.x)).toEqual([0, 1, 2].map((i) => PADDING + i * (CARD_W + GAP_X)));
  expect(chartBounds(flattenLayout([root]))).toEqual({
    width: PADDING + 3 * CARD_W + 2 * GAP_X + PADDING,
    height: PADDING + 2 * CARD_H + GAP_Y + PADDING,
  });
});

test("fit never zooms past 100% and centres the chart", () => {
  expect(fitChartToViewport(1000, 800, { width: 400, height: 200 })).toEqual({ zoom: 1, pan: { x: 300, y: 300 } });
  expect(fitChartToViewport(440, 240, { width: 800, height: 200 })?.zoom).toBe(0.5);
  expect(fitChartToViewport(40, 800, { width: 400, height: 200 })).toBeNull();
});

test("a filter keeps a node that matches or has a report that does", () => {
  const tree = [node(1, [node(2, [], "paused"), node(3)]), node(4)];
  const kept = filterOrgTree(tree, ["paused"]);
  expect(kept.map((n) => [n.id, n.reports.map((r) => r.id)])).toEqual([[1, [2]]]);
  expect(filterOrgTree(tree, ["terminated"])).toEqual([]);
});
