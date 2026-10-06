import { apiFetch } from "./client.js";

export function customerFileDownloadErrorMessage(response, data = {}) {
  if (response.status === 401) {
    return "登录已过期，请先打开 /app/login 重新登录后再下载";
  }
  if (response.status === 403) {
    return "当前账号无权下载此文件";
  }
  return data.error || data.message || "下载失败";
}

export async function downloadCustomerFile(url) {
  const response = await apiFetch(url);
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(customerFileDownloadErrorMessage(response, data));
  }
  const blob = await response.blob();
  const href = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = href;
  link.download =
    response.headers
      .get("Content-Disposition")
      ?.match(/filename="?([^";]+)/)?.[1] || url.split("?")[0].split("/").pop();
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(href), 1000);
}
