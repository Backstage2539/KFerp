# Customer Account Fulfillment Visibility Design

## Goal

Hide the fulfillment operator console from customer-account mode without deleting the page or removing any existing fulfillment APIs.

## Scope

- Add an administrator-controlled UI setting named `hide_customer_account_fulfillment`.
- Default the setting to `true`.
- When the current actor is a customer account and the setting is enabled, the Vue shell menu filters out the internal `customerFulfillment` page.
- Keep `customerFulfillment`, `customerProcessingPortal`, fulfillment order APIs, imports, and manuals available in code.
- Add a settings menu page where administrators can turn the hiding rule on or off.

## Architecture

- Backend: support HTTP module owns `/api/ui-settings`, backed by `app_config`, and writes an audit entry on update.
- Frontend: `App.vue` loads UI settings after authentication and passes the setting into the existing menu filtering helper.
- Settings UI: a small `UISettingsView.vue` page under the Settings menu saves the flag.

## Testing

- Node unit tests cover menu filtering for customer-account actors and API wrapper paths.
- Go API tests cover default settings, persistence, permission rejection, and audit intent through the settings handler.
- Existing full verification still runs: `go test ./...`, frontend node tests, `npm run build`, and `git diff --check`.
