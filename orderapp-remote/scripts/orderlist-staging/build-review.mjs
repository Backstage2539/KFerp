import fs from "node:fs/promises";
import path from "node:path";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const [datasetPath, outputPath, renderDir = ""] = process.argv.slice(2);
if (!datasetPath || !outputPath) {
  throw new Error("usage: build-review.mjs <dataset.json> <output.xlsx> [render-dir]");
}

const dataset = JSON.parse(await fs.readFile(datasetPath, "utf8"));
const workbook = Workbook.create();

const colors = {
  navy: "#17365D",
  blue: "#DCE6F1",
  lightBlue: "#EAF2F8",
  green: "#E2F0D9",
  amber: "#FFF2CC",
  red: "#FCE4D6",
  gray: "#F2F2F2",
  line: "#D9E2F3",
  text: "#1F2937",
  muted: "#667085",
  white: "#FFFFFF",
};

const reviewStatuses = ["auto_ready", "needs_review", "approved", "excluded"];

buildSummarySheet();
buildTableSheet("序号映射", [
  "工作表", "物理行号", "原序号", "重复后缀", "有效序号", "来源订单键", "内容指纹", "订单日期", "审核状态",
], dataset.raw_orders.map((row) => [
  row.source_sheet_name, row.source_row_number, row.source_sequence_original, row.duplicate_suffix,
  row.source_sequence_effective, row.source_order_key, String(row.source_fingerprint || "").slice(0, 16), row.order_date, row.review_status,
]));

buildTableSheet("客户候选", [
  "客户键", "规范名称", "手机号", "联系人", "当前地址原文", "ERP客户ID", "ERP规范名称", "匹配方式", "审核状态", "审核结论", "审核备注",
], sortedRows(dataset.customers, "canonical_name").map((row) => [
  row.customer_key, row.canonical_name, row.normalized_phone, row.current_contact, row.current_address,
  row.erp_match_id || "", row.erp_match_name, row.match_method, row.review_status, "", "",
]));

buildTableSheet("客户别名", [
  "客户键", "历史名称", "规范名称键", "来源订单键", "出现日期",
], sortedRows(dataset.customer_aliases, "alias").map((row) => [
  row.customer_key, row.alias, row.alias_normalized, row.source_order_key, row.observed_date,
]));

buildTableSheet("父商品候选", [
  "父商品键", "规范名称", "商品类型", "烘焙度", "ERP商品ID", "ERP规范名称", "匹配方式", "匹配分", "审核状态", "审核结论", "审核备注",
], sortedRows(dataset.products, "canonical_name", true).map((row) => [
  row.product_key, row.canonical_name, row.product_kind, row.roast_level, row.erp_match_id || "",
  row.erp_match_name, row.match_method, row.match_score, row.review_status, "", "",
]));

buildTableSheet("SKU规格", [
  "SKU键", "父商品键", "规格名称", "销售单位", "净含量", "净含量单位", "单规格克数", "审核状态", "审核结论", "审核备注",
], sortedRows(dataset.skus, "spec_name", true).map((row) => [
  row.sku_key, row.product_key, row.spec_name, row.sales_unit, row.net_content_qty,
  row.net_content_unit, row.normalized_weight_g, row.review_status, "", "",
]));

buildTableSheet("订单候选", [
  "来源订单键", "工作表", "原序号", "有效序号", "物理行", "订单日期", "客户键", "客户原文",
  "订单来源", "订单类型", "付款状态", "发货状态", "金额", "金额原文", "派生金额", "运费", "运费原文", "单号", "备注", "审核状态", "审核结论", "审核备注",
], [...dataset.orders].sort((a, b) => String(b.order_date || "").localeCompare(String(a.order_date || "")) || String(a.source_order_key).localeCompare(String(b.source_order_key), "zh-CN", { numeric: true })).map((row) => [
  row.source_order_key, row.sheet_name, row.sequence_original, row.sequence_effective, row.source_row_number,
  row.order_date, row.customer_key, row.customer_raw, row.order_source_raw, row.order_type_raw,
  row.payment_status_raw, row.shipment_status_raw, row.amount_value, row.amount_raw, row.amount_derived,
  row.shipping_amount_value, row.shipping_amount_raw, row.tracking_no_raw, row.remark_raw, row.review_status, "", "",
]));

buildTableSheet("订单明细", [
  "来源明细键", "来源订单键", "行号", "原始商品行", "父商品键", "SKU键", "父商品名称", "规格名称",
  "商品类型", "烘焙度", "数量", "单位", "总重量g", "审核状态", "审核结论", "审核备注",
], [...dataset.order_items].sort((a, b) => String(a.source_order_key).localeCompare(String(b.source_order_key), "zh-CN", { numeric: true }) || a.line_no - b.line_no).map((row) => [
  row.source_item_key, row.source_order_key, row.line_no, row.raw_line, row.product_key, row.sku_key,
  row.parent_name, row.spec_name, row.product_kind, row.roast_level, row.order_quantity, row.order_unit,
  row.normalized_weight_g, row.review_status, "", "",
]));

const erpMatches = [];
for (const row of dataset.customers) {
  if (row.erp_match_id) {
    erpMatches.push(["客户", row.customer_key, row.canonical_name, row.erp_match_id, row.erp_match_name, row.match_method, 1, row.review_status]);
  }
}
for (const row of dataset.products) {
  if (row.erp_match_id) {
    erpMatches.push(["父商品", row.product_key, row.canonical_name, row.erp_match_id, row.erp_match_name, row.match_method, row.match_score, row.review_status]);
  }
}
buildTableSheet("ERP匹配建议", [
  "实体类型", "候选键", "候选规范名称", "ERP ID", "ERP名称", "匹配方式", "匹配分", "审核状态", "审核结论", "审核备注",
], erpMatches.map((row) => [...row, "", ""]));

buildTableSheet("待审核问题", [
  "问题键", "实体类型", "实体键", "问题代码", "严重度", "说明", "来源订单键", "工作表", "物理行", "审核状态", "处理决定", "审核人", "审核备注",
], [...dataset.issues].sort((a, b) => String(a.code).localeCompare(String(b.code)) || String(a.source_order_key).localeCompare(String(b.source_order_key), "zh-CN", { numeric: true })).map((row) => [
  row.issue_key, row.entity_type, row.entity_key, row.code, row.severity, row.message,
  row.source_order_key, row.sheet_name, row.source_row_number, row.review_status, "", "", "",
]));

buildTableSheet("排除工作表", [
  "工作表", "识别月份", "是否纳入", "排除原因", "使用行数", "订单行数",
], dataset.sheets.filter((row) => !row.included).map((row) => [
  row.sheet_name, row.period, row.included, row.excluded_reason, row.used_row_count, row.order_row_count,
]));

await fs.mkdir(path.dirname(outputPath), { recursive: true });
const summaryInspect = await workbook.inspect({
  kind: "table",
  sheetId: "导入汇总",
  range: "A1:H24",
  include: "values,formulas",
  tableMaxRows: 24,
  tableMaxCols: 8,
  maxChars: 6000,
});
console.log(summaryInspect.ndjson);

const formulaErrors = await workbook.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
  options: { useRegex: true, maxResults: 100 },
  summary: "review workbook formula error scan",
  maxChars: 3000,
});
console.log(formulaErrors.ndjson);

if (renderDir) {
  await fs.mkdir(renderDir, { recursive: true });
  for (const sheetName of reviewSheetNames()) {
    const sheet = workbook.worksheets.getItem(sheetName);
    const used = sheet.getUsedRange(true);
    const rowCount = Math.min(used?.rowCount || 25, 25);
    const colCount = Math.min(used?.columnCount || 8, 12);
    const range = `A1:${columnName(colCount)}${rowCount}`;
    const image = await workbook.render({ sheetName, range, scale: 1.25, format: "png" });
    const renderPath = path.join(renderDir, `${String(reviewSheetNames().indexOf(sheetName) + 1).padStart(2, "0")}-${sheetName}.png`);
    await fs.writeFile(renderPath, new Uint8Array(await image.arrayBuffer()), { mode: 0o600 });
    await fs.chmod(renderPath, 0o600);
  }
}

const output = await SpreadsheetFile.exportXlsx(workbook);
await output.save(outputPath);
await fs.chmod(outputPath, 0o600);

function buildSummarySheet() {
  const sheet = workbook.worksheets.add("导入汇总");
  sheet.showGridLines = false;
  sheet.mergeCells("A1:H1");
  sheet.getRange("A1").values = [["咖啡销售历史数据清洗审核"]];
  sheet.getRange("A1:H1").format = {
    fill: colors.navy,
    font: { name: "Microsoft YaHei", size: 18, bold: true, color: colors.white },
    horizontalAlignment: "left",
    verticalAlignment: "center",
    rowHeight: 38,
  };
  const run = dataset.run;
  const meta = [
    ["运行批次", run.run_id, "源文件", run.source_path],
    ["SHA256", run.source_sha256, "处理范围", `${run.start_period} 至 ${run.end_period}`],
    ["源文件大小", run.source_bytes, "生成时间", run.created_at],
  ];
  sheet.getRange("A3:D5").values = meta;
  sheet.getRange("A3:D5").format = { font: { name: "Microsoft YaHei", size: 10, color: colors.text }, wrapText: true };
  sheet.getRange("D5").format.numberFormat = "yyyy-mm-dd hh:mm:ss";
  sheet.getRange("A3:A5").format = { fill: colors.blue, font: { bold: true, color: colors.navy } };
  sheet.getRange("C3:C5").format = { fill: colors.blue, font: { bold: true, color: colors.navy } };

  const metrics = [
    ["指标", "数量"],
    ["工作表总数", dataset.sheets.length],
    ["纳入月度表", dataset.sheets.filter((row) => row.included).length],
    ["原始订单行", dataset.raw_orders.length],
    ["有效订单候选", dataset.orders.length],
    ["客户候选", dataset.customers.length],
    ["父商品候选", dataset.products.length],
    ["SKU规格", dataset.skus.length],
    ["订单明细", dataset.order_items.length],
    ["待审核问题", dataset.issues.length],
  ];
  sheet.getRange(`A8:B${7 + metrics.length}`).values = metrics;
  styleSummaryBlock(sheet, `A8:B${7 + metrics.length}`);

  const issueCounts = Object.entries(countBy(dataset.issues, "code")).sort((a, b) => b[1] - a[1]);
  const issueRows = [["问题代码", "数量"], ...issueCounts];
  sheet.getRange(`D8:E${7 + issueRows.length}`).values = issueRows;
  styleSummaryBlock(sheet, `D8:E${7 + issueRows.length}`);

  const statusRows = [["订单审核状态", "数量"], ...Object.entries(countBy(dataset.orders, "review_status"))];
  sheet.getRange(`G8:H${7 + statusRows.length}`).values = statusRows;
  styleSummaryBlock(sheet, `G8:H${7 + statusRows.length}`);

  sheet.getRange("A22").values = [["说明：本工作簿仅用于数据审核。approved 数据后续仍需通过 KFerp 服务/API 正式导入；当前未写入正式客户、商品或订单表。"]];
  sheet.getRange("A22:H22").format = {
    font: { name: "Microsoft YaHei", size: 10, color: colors.text },
    wrapText: true,
    verticalAlignment: "center",
    borders: { top: { style: "medium", color: colors.navy } },
  };
  [20, 32, 18, 32, 14, 4, 24, 14].forEach((width, index) => {
    sheet.getRangeByIndexes(0, index, 24, 1).format.columnWidth = width;
  });
}

function styleSummaryBlock(sheet, rangeAddress) {
  const range = sheet.getRange(rangeAddress);
  range.format = {
    font: { name: "Microsoft YaHei", size: 10, color: colors.text },
    borders: { preset: "all", style: "thin", color: colors.line },
  };
  range.getRow(0).format = { fill: colors.navy, font: { bold: true, color: colors.white } };
}

function buildTableSheet(name, headers, rows) {
  const sheet = workbook.worksheets.add(name);
  sheet.showGridLines = false;
  const values = [headers, ...rows.map((row) => row.map(cleanCell))];
  sheet.getRangeByIndexes(0, 0, values.length, headers.length).values = values;
  const header = sheet.getRangeByIndexes(0, 0, 1, headers.length);
  header.format = {
    fill: colors.navy,
    font: { name: "Microsoft YaHei", size: 10, bold: true, color: colors.white },
    horizontalAlignment: "center",
    verticalAlignment: "center",
    wrapText: true,
    rowHeight: 30,
    borders: { preset: "outside", style: "medium", color: colors.navy },
  };
  if (rows.length > 0) {
    const body = sheet.getRangeByIndexes(1, 0, rows.length, headers.length);
    body.format = {
      font: { name: "Microsoft YaHei", size: 9, color: colors.text },
      verticalAlignment: "top",
      wrapText: true,
      rowHeight: 34,
      borders: { insideHorizontal: { style: "thin", color: colors.line } },
    };
    for (let col = 0; col < headers.length; col += 1) {
      const width = columnWidth(headers[col]);
      sheet.getRangeByIndexes(0, col, rows.length + 1, 1).format.columnWidth = width;
    }
    const statusIndex = headers.indexOf("审核状态");
    if (statusIndex >= 0) {
      const statusRange = sheet.getRangeByIndexes(1, statusIndex, rows.length, 1);
      statusRange.dataValidation = { rule: { type: "list", values: reviewStatuses } };
      statusRange.format.fill = colors.amber;
    }
    const decisionIndex = headers.findIndex((value) => value === "审核结论" || value === "处理决定");
    if (decisionIndex >= 0) {
      const decisionRange = sheet.getRangeByIndexes(1, decisionIndex, rows.length, 1);
      decisionRange.dataValidation = { rule: { type: "list", values: ["approved", "excluded", "keep", "edit"] } };
      decisionRange.format.fill = colors.lightBlue;
    }
  }
  sheet.freezePanes.freezeRows(1);
  sheet.freezePanes.freezeColumns(Math.min(2, headers.length));
}

function countBy(rows, field) {
  const result = {};
  for (const row of rows) {
    const key = row[field] || "未设置";
    result[key] = (result[key] || 0) + 1;
  }
  return result;
}

function sortedRows(rows, field, blanksLast = false) {
  return [...rows].sort((a, b) => {
    const av = String(a[field] || "");
    const bv = String(b[field] || "");
    if (blanksLast && !av !== !bv) return av ? -1 : 1;
    return av.localeCompare(bv, "zh-CN", { numeric: true });
  });
}

function cleanCell(value) {
  if (value === undefined || value === null) return null;
  if (typeof value === "object") return JSON.stringify(value);
  return value;
}

function columnWidth(header) {
  if (/原文|地址|备注|说明|原始商品行/.test(header)) return 38;
  if (/指纹|来源.*键|客户键|商品键|SKU键|问题键|实体键/.test(header)) return 26;
  if (/名称|工作表|匹配方式|问题代码/.test(header)) return 20;
  if (/日期|状态|类型|单位|序号/.test(header)) return 14;
  if (/数量|金额|运费|行号|后缀|匹配分|重量|净含量/.test(header)) return 12;
  return 16;
}

function columnName(columnCount) {
  let n = columnCount;
  let out = "";
  while (n > 0) {
    n -= 1;
    out = String.fromCharCode(65 + (n % 26)) + out;
    n = Math.floor(n / 26);
  }
  return out;
}

function reviewSheetNames() {
  return ["导入汇总", "序号映射", "客户候选", "客户别名", "父商品候选", "SKU规格", "订单候选", "订单明细", "ERP匹配建议", "待审核问题", "排除工作表"];
}
