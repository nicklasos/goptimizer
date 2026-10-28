# Add a Playwright test

The suite runs against a real gogo API on the test database. It starts both servers itself
(`make test-e2e`); do not start them by hand.

## 1. Pick the folder

| Folder | Session | Use it for |
|---|---|---|
| `tests/e2e/shared-session/` | One plain user, signed in once for the whole run | Flows any user can do, that do not change the account itself |
| `tests/e2e/own-session/` | None; the spec signs in itself | Anything role-specific, anything that changes the account (password, profile), and signed-out pages |

A spec runs because of the folder it is in. There is nothing to register.

## 2. Data

- `loginAs(page, 'admin')` creates a user with that role and signs the page in. It returns the user.
- `dbHelper.createUser({ roles, emailVerified })` and `dbHelper.createEmailToken(userId, 'password_reset')` seed the database directly.
- Records created through the UI must be findable by cleanup: emails from `uniqueEmail('label')`, names starting with `E2E`.
- Add a method to `tests/e2e/helpers/db-helper.ts` when a new table needs seeding.

## 3. Page objects

Put locators and multi-step actions in `tests/e2e/pages/<Name>Page.ts`, extending `BasePage`. Specs read as steps, not selectors.

## 4. Locators

| To find | Use |
|---|---|
| Any control | `page.getByTestId('order-save-button')` |
| A table row | `page.getByTestId('orders-table').locator('tr[data-row-key]').filter({ hasText })` |
| A row's button | `row.locator('[data-testid^="edit-order-button-"]')` |
| The delete confirmation | `page.locator('[data-testid^="confirm-delete-order-button-"]')` |
| A field in error | `expect(input).toHaveAttribute('aria-invalid', 'true')` |

Never assert on visible copy (it is translated) or on Ant Design class names. A `Modal` or `Result` wrapper
has no visible box of its own: assert on a control inside it.

## 5. What to cover

The main path, what a user who is not allowed sees, and one failure the user can cause (a validation error).
To test how the UI reacts to an API answer that is hard to produce, intercept it with `page.route(...)`.

## 6. Run

```bash
npx playwright test tests/e2e/own-session/orders.spec.ts   # one file while writing
make test                                                   # everything before finishing
```

## Check

- The spec passes twice in a row (no leftover data dependence).
- It still passes when run alone and when run with the whole suite.
