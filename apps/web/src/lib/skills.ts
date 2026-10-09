// The agents context's Skills: a Guild's library of Claude skill packages,
// each a SKILL.md and its Skill files, that its people give to Agents. Every
// call answers in the Current guild. splitFrontmatter and buildTree are
// Paperclip's (ui/src/pages/CompanySkills.tsx; MIT, see NOTICE).
import type { AgentStatus } from "@bakery/ui/agentStatus";
import { api } from "./api";

export type SkillFileKind = "skill" | "markdown" | "script" | "other";
export type SkillFileEncoding = "utf8" | "base64";

export type Skill = {
  id: number;
  slug: string;
  name: string;
  description: string;
  file_count: number;
  /** The bytes of all its files. */
  size: number;
  agents_count: number;
  created_at: string;
  updated_at: string;
  created_by: { id: number; name: string } | null;
};

export type SkillFileEntry = {
  path: string;
  kind: SkillFileKind;
  size: number;
  encoding: SkillFileEncoding;
  executable: boolean;
};

export type SkillDetail = Skill & {
  files: SkillFileEntry[];
  /** The Agents that are not terminated and have it. */
  agents: { id: number; name: string; icon: string; status: AgentStatus }[];
};

export type SkillFile = {
  path: string;
  content: string;
  encoding: SkillFileEncoding;
  executable: boolean;
  size: number;
};

/** The one file every Skill has, which cannot be deleted. */
export const skillMarkdown = "SKILL.md";

export const listSkills = () =>
  api<{ skills: Skill[] }>("GET", "/skills").then((r) => r.skills);
export const getSkill = (id: number) =>
  api<{ skill: SkillDetail }>("GET", `/skills/${id}`).then((r) => r.skill);
export const createSkill = (input: {
  name: string;
  slug?: string;
  description?: string;
  markdown?: string;
}) =>
  api<{ skill: SkillDetail }>("POST", "/skills", input).then((r) => r.skill);
export const deleteSkill = (id: number) =>
  api<void>("DELETE", `/skills/${id}`);
export const getSkillFile = (id: number, path: string) =>
  api<{ file: SkillFile }>(
    "GET",
    `/skills/${id}/file?path=${encodeURIComponent(path)}`,
  ).then((r) => r.file);
export const writeSkillFile = (
  id: number,
  file: {
    path: string;
    content: string;
    encoding?: SkillFileEncoding;
    executable?: boolean;
  },
) =>
  api<{ skill: SkillDetail }>("PUT", `/skills/${id}/files`, file).then(
    (r) => r.skill,
  );
export const deleteSkillFile = (id: number, path: string) =>
  api<{ skill: SkillDetail }>(
    "DELETE",
    `/skills/${id}/files?path=${encodeURIComponent(path)}`,
  ).then((r) => r.skill);

/**
 * The Skill slug the API derives from a name: lowercased, every run of other
 * characters a `-`, trimmed to 64.
 */
export function skillSlug(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64)
    .replace(/-+$/, "");
}

/** A Skill file's frontmatter and the Markdown after it. */
export function splitFrontmatter(markdown: string): {
  frontmatter: string | null;
  body: string;
} {
  const normalized = markdown.replace(/\r\n/g, "\n");
  if (!normalized.startsWith("---\n")) return { frontmatter: null, body: normalized };
  const closing = normalized.indexOf("\n---\n", 4);
  if (closing < 0) return { frontmatter: null, body: normalized };
  return {
    frontmatter: normalized.slice(4, closing).trim(),
    body: normalized.slice(closing + 5).trimStart(),
  };
}

/** The frontmatter's top-level `key: value` lines, quotes taken off. */
export function frontmatterFields(frontmatter: string): [string, string][] {
  const fields: [string, string][] = [];
  for (const line of frontmatter.split("\n")) {
    const m = /^([A-Za-z0-9_-]+):\s*(.*)$/.exec(line);
    if (!m) continue;
    fields.push([m[1], m[2].replace(/^(["'])(.*)\1$/, "$2")]);
  }
  return fields;
}

export type SkillTreeNode = {
  name: string;
  /** A file's path, or a folder's path from the Skill's root. */
  path: string;
  kind: "dir" | "file";
  file?: SkillFileEntry;
  children: SkillTreeNode[];
};

/** The Skill's files as folders and files: folders first, SKILL.md on top, then by name. */
export function buildTree(entries: SkillFileEntry[]): SkillTreeNode[] {
  const root: SkillTreeNode = { name: "", path: "", kind: "dir", children: [] };
  for (const entry of entries) {
    const segments = entry.path.split("/").filter(Boolean);
    let current = root;
    let currentPath = "";
    segments.forEach((segment, i) => {
      currentPath = currentPath ? `${currentPath}/${segment}` : segment;
      const leaf = i === segments.length - 1;
      let next = current.children.find((c) => c.name === segment);
      if (!next) {
        next = {
          name: segment,
          path: leaf ? entry.path : currentPath,
          kind: leaf ? "file" : "dir",
          file: leaf ? entry : undefined,
          children: [],
        };
        current.children.push(next);
      }
      current = next;
    });
  }
  const sort = (node: SkillTreeNode) => {
    node.children.sort((a, b) => {
      if (a.kind !== b.kind) return a.kind === "dir" ? -1 : 1;
      if (a.name === skillMarkdown) return -1;
      if (b.name === skillMarkdown) return 1;
      return a.name.localeCompare(b.name);
    });
    node.children.forEach(sort);
  };
  sort(root);
  return root.children;
}

/** The folders a path sits in, outermost first. */
export function parentFolders(path: string): string[] {
  const segments = path.split("/").filter(Boolean);
  return segments.slice(0, -1).map((_, i) => segments.slice(0, i + 1).join("/"));
}

/** Paperclip's formatBytes. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
