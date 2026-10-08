import test from "node:test";
import assert from "node:assert/strict";
import {
  newPageDraft,
  pageLinks,
  groupPageTargets,
  movePageBlock,
} from "./page-entries.js";
test("manual draft defaults and supported destinations", () => {
  const d = newPageDraft();
  assert.equal(d.visibility, "authenticated");
  assert.equal(d.publication_id, 0);
  const p = pageLinks(
    { key: "abc", draft: { kind: "article" } },
    "https://dev.qacoohee.com",
  );
  assert.equal(p.mini, "pages/page-entry/page-entry?entry=abc");
  assert.equal(p.web, "https://dev.qacoohee.com/app/p/abc");
  assert.equal(pageLinks({ key: "abc", draft: { kind: "function" } }).web, "");
});
test("table versions are grouped by stable target rather than caption", () => {
  const groups = groupPageTargets([
    { id: 1, table_scope: "a", name: "标准", version: "v1" },
    { id: 2, table_scope: "a", name: "改名", version: "v2" },
    { id: 3, table_scope: "b", name: "标准", version: "v1" },
  ]);
  assert.equal(groups.length, 2);
  assert.equal(groups[0].versions.length, 2);
});
test("article reordering is local and bounded", () => {
  const blocks = [{ text: "a" }, { text: "b" }];
  assert.deepEqual(movePageBlock(blocks, 0, 1), [{ text: "b" }, { text: "a" }]);
  assert.deepEqual(blocks, [{ text: "a" }, { text: "b" }]);
  assert.deepEqual(movePageBlock(blocks, 0, -1), blocks);
});

test("menu selection changes only on explicit action and obeys page modes", async () => {
  const { setMenuPage, menuPageKey, menuAction, menuToEditor, menuFromEditor } =
    await import("./wechat-official.js");
  const key = "0123456789abcdef0123456789abcdef",
    b = { name: "豆单", type: "view", url: "https://example.test/legacy" };
  const e = { key, published: { kind: "price", name: "标准版" } };
  setMenuPage(b, e, "mini-app", "web", "https://dev.qacoohee.com");
  assert.equal(menuAction(b), "page");
  assert.equal(menuPageKey(b), key);
  assert.equal(b.url, "https://dev.qacoohee.com/app/p/" + key);
  assert.deepEqual(menuFromEditor(menuToEditor({ button: [b] })), {
    button: [b],
  });
  setMenuPage(
    b,
    { ...e, published: { kind: "function" } },
    "mini-app",
    "web",
    "https://dev.qacoohee.com",
  );
  assert.equal(b.type, "miniprogram");
  assert.equal(b.pagepath, "pages/page-entry/page-entry?entry=" + key);
});
