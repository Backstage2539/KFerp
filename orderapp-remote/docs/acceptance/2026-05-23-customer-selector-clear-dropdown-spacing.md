# PR-338-CUSTOMER-SELECTOR-CLEAR-DROPDOWN-SPACING 验收

## 需求

- 客户账户顶部“当前客户”选择器已有客户值时，清除选择按钮和展开下拉按钮不得重叠。
- 两个按钮必须保留独立点击区域：清除用于清空当前客户，展开用于打开候选客户列表。
- 清除后选择器恢复为空值，可继续输入搜索或点下拉重新选择客户。

## 证据

- RED：`node --test src/lib/searchable-select.test.js` 中 `workspace customer selector keeps clear and dropdown controls in separate hit targets` 先失败，暴露缺少独立清除按钮、仍使用原生 search 清除控件、顶部客户选择器右侧 padding 不足。
- GREEN：同一测试通过；`SearchableSelect.vue` 使用显式 `select-clear` 按钮，输入框为 `type="text"`，右侧保留 70px；顶部客户选择器同步保留 70px 右内边距。

## 验收点

- 进入客户账户并选择任意客户后，顶部当前客户输入框右侧能看到清除和下拉两个分开的按钮。
- 点击清除按钮不会误触下拉按钮；点击下拉按钮不会误触清除按钮。
- 清除后可继续搜索或重新展开客户候选项。
