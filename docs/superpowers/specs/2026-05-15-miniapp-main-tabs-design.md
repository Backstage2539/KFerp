# Miniapp Main Tabs Design

## Requirement

Customers reported that the published WeChat mini program can open to a blank page with only the top-left home icon, and only enters the login UI after tapping that icon. The mini program also needs a simpler customer-facing main navigation:

- Bottom four large entries: 首页, 订单, 账单, 我的.
- 首页 keeps the current service/home content, but does not duplicate the order entry.
- 订单 is the dedicated order area. It loads order history, order line details, sales order information, and outbound/logistics information.
- 账单 is the dedicated settlement area. It loads fee items and settlement batches.
- 我的 contains logout and account switching now, and remains the future place for personal settings.

## Design

Add a lightweight startup page at `pages/index/index` and make it the first page in `pages.json`. The startup page reads the stored token through the existing Pinia session store and uses `uni.reLaunch` to send unauthenticated users to `/pages/login/login` and authenticated users to `/pages/home/home`. This avoids relying on the WeChat top-left home control and prevents stale page stacks from showing a blank root page.

Introduce a shared bottom navigation component `src/components/MainTabBar.vue`. It renders the four main entries on every authenticated top-level page: home, service/order, service/settlement, and profile. The component uses `uni.reLaunch` for main-tab navigation so each tab has a predictable page stack. It highlights the active tab through a `current` prop.

Keep the existing service detail page as the data-loading surface for orders and settlement:

- `/pages/service/service?key=orders` becomes the 订单 tab.
- `/pages/service/service?key=settlement` becomes the 账单 tab.
- `/pages/home/home` remains 首页 and filters out the order and settlement entries from the service grid.
- `/pages/profile/profile` becomes 我的.

Remove the top "个人中心" buttons from home, mall, and service pages. My/account actions live only under the 我的 tab. Login success and token-expired redirects should use `uni.reLaunch` so the user cannot return to a blank or stale unauthenticated page.

## Testing

Miniapp unit/source tests should cover:

- `pages/index/index` is first in `pages.json`.
- Startup page routes token/no-token states to home/login through `uni.reLaunch`.
- Home, service, mall, and profile pages include `MainTabBar`.
- Header "个人中心" button is removed from authenticated content pages.
- Home grid excludes the order and settlement service cards.
- Orders and settlement API paths continue to use `/api/mini/services/orders` and `/api/mini/services/settlement`.

Backend API coverage remains on existing `/api/mini/services/orders` and `/api/mini/services/settlement` tests, because this change is primarily miniapp navigation and presentation.

## Documentation

Update `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, both customer portal operation manuals, and the PR/DEV/UT/API/REV seed table with `PR-278-MINIAPP-MAIN-TABS`.
