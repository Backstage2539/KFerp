import { parseRecipientText } from "./customer-recipient.js";
export function mergeRecipient(current, text) {
  const parsed = parseRecipientText(text);
  // A phone-free address block cannot reliably identify a recipient name.
  return {
    receiver_name:
      (parsed.phone && parsed.recipient_name) || current.receiver_name || "",
    receiver_phone: parsed.phone || current.receiver_phone || "",
    receiver_address: parsed.address || current.receiver_address || "",
  };
}
export function togglePageSelection(selected, rows) {
  const ids = rows.filter((r) => !r.is_void).map((r) => Number(r.id));
  const set = new Set(selected);
  const remove = ids.length > 0 && ids.every((id) => set.has(id));
  for (const id of ids) {
    if (remove) set.delete(id);
    else set.add(id);
  }
  return [...set];
}
export function customerAccountView(key) {
  return (
    {
      financeExpenses: "customerOrderFees",
      financeClosing: "customerSettlement",
      financeReport: "customerSettlement",
    }[key] || key
  );
}
export function accountStatus(value) {
  return (
    {
      unpaid: "未付款",
      partial: "部分付款",
      paid: "已付款",
      confirmed: "已确认",
      draft: "待确认",
      reversed: "已冲销",
      settled: "已入结算，付款待核实",
      unknown: "待核实",
    }[value] ||
    value ||
    "未入结算"
  );
}
export function accountFeeType(value) {
  return (
    {
      processing: "加工费",
      packaging: "包装费",
      direct_ship_service: "代发服务费",
      storage: "仓储费",
      shipping: "运费",
      adjustment: "调整",
    }[value] || value
  );
}
