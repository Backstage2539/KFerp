import test from "node:test";
import assert from "node:assert/strict";
import {
  mergeRecipient,
  togglePageSelection,
  customerAccountView,
} from "./customer-account.js";
import { customerWorkspaceMenu } from "./customer-workspace.js";
test("recipient parsing preserves manually entered fields when absent", () => {
  assert.deepEqual(
    mergeRecipient(
      {
        receiver_name: "原姓名",
        receiver_phone: "13800000000",
        receiver_address: "原地址",
      },
      "新地址 上海市徐汇区测试路1号",
    ),
    {
      receiver_name: "原姓名",
      receiver_phone: "13800000000",
      receiver_address: "新地址 上海市徐汇区测试路1号",
    },
  );
  const r = mergeRecipient({}, "张三 13812345678 上海市徐汇区测试路1号");
  assert.equal(r.receiver_phone, "13812345678");
  assert.equal(r.receiver_name, "张三");
});
test("page selection preserves other pages and does not select void orders", () => {
  const rows = [{ id: 2 }, { id: 3, is_void: true }];
  assert.deepEqual(togglePageSelection([1], rows), [1, 2]);
  assert.deepEqual(togglePageSelection([1, 2], rows), [1]);
});
test("external finance routes and menu resolve to own trading pages", () => {
  assert.equal(customerAccountView("financeExpenses"), "customerOrderFees");
  assert.equal(customerAccountView("financeClosing"), "customerSettlement");
  assert.equal(customerAccountView("financeReport"), "customerSettlement");
  const keys = customerWorkspaceMenu(["settlement", "direct_ship"]).flatMap(
    (g) => g.items.map((i) => i.key),
  );
  assert.ok(keys.includes("customerOrderFees"));
  assert.ok(!keys.includes("financeReport"));
});
