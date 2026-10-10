export const pageKinds = {
  price: "价格表",
  function: "现有功能页",
  article: "自定义图文页",
};
export function newPageDraft() {
  return {
    name: "",
 bean_center:false,
 bean_sort:0,
    kind: "price",
    visibility: "authenticated",
    publication_id: 0,
    target: "",
    blocks: [],
  };
}
export function pageLinks(entry, origin = "") {
  const kind = entry.published?.kind || entry.draft?.kind;
  return {
    mini: `pages/page-entry/page-entry?entry=${encodeURIComponent(entry.key)}`,
    web:
      kind === "function"
        ? ""
        : `${origin}/app/p/${encodeURIComponent(entry.key)}`,
  };
}
export function groupPageTargets(rows) {
  const groups = new Map();
  for (const row of rows) {
    if (!groups.has(row.table_scope))
      groups.set(row.table_scope, { ...row, versions: [] });
    groups.get(row.table_scope).versions.push(row);
  }
  return [...groups.values()];
}
export function movePageBlock(blocks, index, delta) {
  const copy = [...blocks],
    next = index + delta;
  if (next < 0 || next >= copy.length) return copy;
  [copy[index], copy[next]] = [copy[next], copy[index]];
  return copy;
}
export function pageState(entry) {
  return entry.enabled
    ? entry.has_draft
      ? "已发布 · 有新草稿"
      : "已发布"
    : entry.published
      ? "已停用"
      : "草稿";
}
