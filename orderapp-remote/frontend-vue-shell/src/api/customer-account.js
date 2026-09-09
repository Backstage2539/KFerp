import { apiFetch } from "./client";
export async function downloadCustomerFile(url) {
  const response = await apiFetch(url);
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    throw new Error(data.error || data.message || "下载失败");
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
