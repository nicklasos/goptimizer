# Goptimizer frontend – Agent Guide

## Rules
- No obvious or redundant comments.
- **Do not start the dev server** — the user runs it at http://localhost:5747.
- Every interactive element gets a `data-testid` (see Test locators).
- Finish with `make test` (typecheck, lint, unit, E2E) and fix what fails.

## Start here
- **Recipes**: `docs/recipes/` has step-by-step procedures: build a feature across the API and this UI, add a feature, add a Playwright test. Use the matching one instead of improvising.
- **Reference feature**: `src/features/users`.
- **The API** is `../backend`. Its OpenAPI file becomes `src/api/schema.ts` through `make api-types`.

## Role
The admin UI of Goptimizer, for the Go API in `../backend`.

## Stack
- React 19, Vite, TypeScript (strict), Ant Design 5
- TanStack Query for server state, zustand only for the auth session
- react-router-dom (declarative `<Routes>`), i18next (en, uk)
- Native `fetch` through `src/api/client.ts` (no axios)
- Unit: `node:test` through `tsx` | E2E: Playwright with page objects, in TypeScript

## Structure
```
src/
  main.tsx  index.css  vite-env.d.ts
  app/       App, providers, queryClient, router, theme, i18n
  api/       client, errors, types, queryKeys, url
  auth/      authStore, guards (RequireAdmin), LoginPage
  layout/    AppShell, nav (menu registry), navMatch
  shared/    components/, hooks/, utils/, lib/unsavedChanges/
  features/  <module>/{types,api,hooks}.ts + pages/ (+ components/)
  locales/{en,uk}/translation.json
tests/e2e/   specs, pages/ (page objects), helpers/
```
Import with the `@/` alias. A feature imports from `api`, `auth`, `layout` and `shared`, never from another feature.

## Adding a module
1. Run `make swagger` in `../backend` and `make api-types` here, then give the generated types short names in `features/<module>/types.ts` (`export type ManagedUser = InternalUsersUserResponse`). Do not hand-write a type the backend already describes, and never edit `src/api/schema.ts`.
2. `features/<module>/api.ts` — an `xApi` object of calls built on `api` from `@/api/client`.
3. `features/<module>/hooks.ts` — `useX` queries, `useSaveX({ id?, body })`, `useDeleteX`. Mutations invalidate a key prefix from `qk`.
4. Add the keys to `api/queryKeys.ts`.
5. `features/<module>/pages/*.tsx` — pages call hooks only, never `api` directly.
6. Add the route to `app/router.tsx` as a `lazy(() => import(...))` page (default export) and the menu entry to `TOP_NAV` or `USER_MENU` in `layout/nav.tsx`.
7. Add locale keys to both `en` and `uk`; a unit test fails when the two files have different keys.

`features/users` is the reference: a paginated table with modal forms.

## Folders and settings
- `/` is the dashboard (`features/folders/pages/FoldersPage`, test id `dashboard-page`): overview tiles, the 30-day chart, then one card per folder.
- Overview tiles (`components/FolderStats`, `overview-<tile>` with the figure in `data-value`): bytes saved (with the percent of the original size under it, `<tile>-saved-caption`), optimized, waiting (pending), failed, optimizing now, scans queued or running. `folderTotals` in `features/folders/overview.ts` adds them up from `GET /folders`; the folder page shows the same tiles for one folder (`folder-stats-<tile>`). Failed turns red with an icon only above 0.
- Polling: `pollInterval(folders)` in `overview.ts` is the one rule: 2 s while a scan is queued or running or an image is optimizing, 10 s while an enabled folder with every tool has pending images, off otherwise. `useFolders` uses it, `useFolder(id, { live: true })` on the folder page too (the editor leaves `live` off, so a refetch never resets its form), and the chart and scans table refetch at the same rate.
- The chart (`components/SavingsCard`) is hand-rolled SVG over `GET /daily-savings` (30 UTC days, today last), all folders on `/` and one on its page: one column per day in `colorPrimary`, round byte ticks from `byteTicks` (`savings.ts`), a tooltip on hover and on focus (the arrow keys move between days), and each day's text as its `aria-label`. Each day is `savings-day-<YYYY-MM-DD>` with `data-bytes`, `data-before` and `data-count`; the card header and the tooltip show the size before, after and the percent saved.
- Each card's name links to `/folders/:id` (`folder-link-<id>`). It shows how many images the index holds (every status but gone: `indexedImages` in `features/folders/scans.ts`), one line about the folder's `last_scan` (`scanLine`), how many images are optimized with `bytes_saved` out of `bytes_before` and the percent (`folder-optimized-<id>`), the failed count when there are any (`folder-failed-<id>`), and a warning with a link to `/settings` when `missing_tools` is not empty (`folder-missing-tools-<id>`, `MissingToolsAlert`).
- The pause switch, Scan now and Edit come from `useFolderControls(folder, buttonType)`, which the card and the folder page place where they want. Scan now (`scan-folder-button-<id>`, then `quick-scan-item-<id>` or `full-scan-item-<id>`) queues a scan; it is disabled while one is queued or running and for a paused folder.
- `/folders/:id` is `FolderPage`: settings summary and controls, the tiles and chart for that folder, its images (`components/FolderImages`) and its scans (`components/FolderScans`, paginated through `usePageParams`). An id that names no folder shows `page-not-found`.
- Images: one tab per status with its count from `image_counts` (`images-tab-<status>`, a select on phones), a path search (`images-search-input`) and an order (`images-sort-select`; most saved only for optimized, a search lists by path). `readImageFilters` / `writeImageFilters` in `images.ts` keep `status`, `q` and `sort` in the URL and drop what the API would refuse; a write reads the current address, since React Router hands an updater the params of an older render. `useFolderImages` is an infinite query over the keyset pages (`images-load-more-button`); the search box writes the URL 300 ms after typing stops. `IMAGE_STATUS` is the status registry (order and tag colour); `imageSizes` gives before, after and the saving. Rows: `tr[data-row-key]`; status `image-status-<id>` (`data-status`).
- Reasons: `REASON_CODES` in `reasons.ts` maps every code of the API's `ReasonResponse['code']` union to `images.reasons.<code>`, so a new code fails `tsc` until it has a sentence, and `reasons.test.ts` fails until both locales have it. `explainImage` words the reason, the lossless attempt after a refused lossy result, and for a failed image when it is retried or that it gave up; a stored reason without a known code shows as stored. The cell is `image-reason-<id>` with `data-reason-code` and `data-lossless-code`.
- Retry (`retry-image-button-<id>`) and Retry all failed (`retry-failed-images-button`) invalidate `qk.folders.all`: the folder, its images and scans (keys under `qk.folders.detail(id)`) and the list.
- `/folders/new` and `/folders/:id/edit` are `FolderEditorPage`, the long-form pattern: `useUnsavedChangesGuard`, `useGuardedNavigate`, `applyFormErrors`. Form field names are the API's paths (`['profile', 'jpeg_max_quality']`), so server errors land under their fields. Lossy-only fields are not rendered in lossless mode, so the page submits `form.getFieldsValue(true)`.
- `NEW_FOLDER` in `features/folders/folderForm.ts` mirrors the API's default profile and intervals; change both together.
- `/settings` (`features/settings/pages/SettingsPage`) lists the image tools from `GET /tools` and saves the global settings. A tool's purpose is translated by `settings.purpose.<name>`, with the API's English text as the fallback.
- Folders and settings are open to every signed-in user. Only `/users` is for admins.

## API client
- `api.get/post/put/patch/del<T>()` return the response's `data` field.
- `api.getPage<T>()` returns `{ data, pagination }` for paginated lists.
- A non-2xx response throws `ApiError` (`status`, `errorKey`, `message`, `fieldErrors`).
- A 401 triggers one shared token refresh and one retry. `skipAuthRefresh` turns that off (used by the auth calls).
- `client.ts` never imports the auth store; the store registers itself with `bindAuth`.

## Errors
- Queries and mutations show a toast from the global handler in `app/queryClient.ts`. Pass `meta: { silent: true }` when the page shows the error itself.
- The backend sends `error_key`; `shared/utils/serverErrors.ts` translates it through `errors.<key>` in the locales. Add a locale entry for every new backend key.
- Validation errors arrive as `errors: { field: [keys] }`. In a mutation's `onError`, `applyFormErrors(form, t, error, FIELDS)` shows them under their fields and returns the message for anything it could not place (or null).

## Pagination
`usePageParams()` keeps `page` / `page_size` in the URL; `useTablePagination(paging, query.data)` turns the response into the `pagination` prop of `ResponsiveTable`. Without that prop `ResponsiveTable` pages in the browser.

## Admins and users
- An account is an admin or a plain user: `is_admin` on the session user (`useCurrentUser()`). Admins also manage accounts; everything else is open to every user.
- Routes: wrap an admins-only page in `<RequireAdmin>`, which renders `<Forbidden />`.
- Menus: `TOP_NAV` (the top bar) and `USER_MENU` (under the user's name, above Sign out) in `layout/nav.tsx`; an item with `adminOnly: true` is shown to admins alone. The top bar highlights the item with the longest matching path; a page from the user menu highlights nothing there. On phones the top bar's items move into a drawer behind a menu button.
- Guards only hide UI. The backend enforces access.
- User management: `/users` (`features/users/pages/UsersPage.tsx`), reached from the user menu by admins. One paginated table of every account with an Admin tag; the create and edit forms have an Admin switch. An admin's own row offers no delete and their own Admin switch is disabled (`canDeleteOrDemote`).
- Language: the Profile page's picker (`shared/utils/languages.ts` lists the languages). i18next saves the choice in `localStorage` and `app/i18n.ts` mirrors it on `<html lang>`.

## Policies
- Who may do what to a record lives in `features/<module>/policy.ts` as pure functions, such as `canDeleteOrDemote(actor, target)`.
- They mirror the API's `internal/<module>/policy.go` and only decide what the UI offers (a hidden button, a disabled action). The API enforces the same rule and is the authority.
- The route guard (`RequireAdmin`) is for "this page is for admins". A policy is for rules that depend on the record.
- Every policy has cases in `src/features/policies.test.ts`, the same cases as the API's policy test.

## Crashes and unknown URLs
- `AppShell` wraps the routed page in an `ErrorBoundary` keyed by path and a `Suspense`: a render error shows `CrashScreen` in the content area, and navigating elsewhere clears it. `App` has a second, full-screen boundary for everything else.
- A boundary only catches render errors. Failed requests are handled by the query layer (see Errors).
- An unmatched URL renders `NotFoundPage` inside the shell; do not redirect unknown paths.

## Signed-out pages
`app/router.tsx` has two route trees: `PublicRoutes` (login, forgot/reset password, verify email) and `AppRoutes` (everything inside `AppShell`). Signed-out pages sit in `auth/AuthCard`. Any other URL shows the login form in place, so signing in lands on the page that was asked for.

## Styling
Use AntD components and tokens (`theme.useToken()`); no literal colours or ad-hoc CSS. Branding lives in `app/theme.ts`. Page skeleton: `PageStack` > `Card` > `PageHeader` + content.

## Test locators (required)
- Naming: `<entity>-<element>[-<id>]`, for example `add-user-button`, `user-email-input`, `set-password-user-button-12`, `users-table`.
- AntD `Result` and `Modal` do not give the test id a visible box: wrap `Result` in a `div`, and assert on a control inside a modal.
- E2E: page objects in `tests/e2e/pages/`, `getByTestId` first. Table rows: `tr[data-row-key]`. Field errors: `aria-invalid="true"` on the input.
- No assertions on copy or on Ant Design class names.

## Commands
```bash
make typecheck
make lint         # typecheck + eslint
make test-unit    # node:test
make test-e2e     # Playwright: starts the API with --test-db on 8748 and Vite on 5748
make test         # all of the above
make api-types    # regenerate src/api/schema.ts from ../backend/docs/swagger.json
make build
```

## E2E notes
- `tests/e2e/shared-session/`: specs that share one signed-in plain user (`auth.setup.ts` writes `.auth/user.json`).
- `tests/e2e/own-session/`: specs that sign in themselves with `loginAs(page, 'admin' | 'user')`, or test the signed-out pages.
- A spec runs in the project of the folder it is in; nothing has to be registered.
- Users are seeded straight into the test database (`helpers/db-helper.ts`); emails start with `e2e-` so cleanup finds them. Use `uniqueEmail()` for users created through the UI.
- Folders made through the UI are named `E2E ...` so cleanup finds them, and point at a directory the spec creates under `os.tmpdir()` (the API runs on the same machine). The folders table is shared by every spec, so never assert on the whole list, the empty state or the dashboard's totals.
- `dbHelper.createFolder` seeds a paused folder, which the test API's scheduler and optimizer leave alone; `createImages` (one transaction, `updated_at` written as the API writes it, since the keyset compares the text), `createScans` and `createDailyStats` fill it. Seed a failed image with a future `retryInMinutes` or none: a due retry is requeued even in a paused folder.
- The API is started with `AUTH_RATE_LIMIT=false` and `APP_DEBUG=true` (no throttling, mail only logged), after migrating the test database: its scan worker reads the `scans` table at boot, and Playwright starts it before `globalSetup`.
- The test API runs the scheduler and the scan worker, so a folder a spec creates gets its first scan within a minute. A spec that clicks Scan now accepts 409 `scans.in_progress` for the rare run where the schedule queued one first.
- Emailed links cannot be read back (only a hash is stored): plant a token with `dbHelper.createEmailToken(userId, purpose)`.
- Requires `TEST_DATABASE_PATH` (a SQLite file whose name ends in `_test.db`, relative to `../backend`) in `../backend/.env`. The helper opens the same file the API writes.

## Don’t
- Don’t call `fetch` or `api` from a page; go through `features/<module>/hooks.ts`.
- Don’t start `npm run dev` unless the user asks.
