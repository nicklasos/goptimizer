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
- `/` is the dashboard: one card per folder (`features/folders/pages/FoldersPage`), with a pause switch and an edit link. It keeps the test id `dashboard-page`.
- Each card shows how many images the index holds (every status but gone: `indexedImages` in `features/folders/scans.ts`) and one line about the folder's `last_scan`. Scan now (`scan-folder-button-<id>`, then `quick-scan-item-<id>` or `full-scan-item-<id>`) queues a scan; it is disabled while one is queued or running and for a paused folder. `useFolders` refetches every 2 s while any folder's scan is queued or running, and not at all otherwise.
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
- Folders made through the UI are named `E2E ...` so cleanup finds them, and point at a directory the spec creates under `os.tmpdir()` (the API runs on the same machine). The folders table is shared by every spec, so never assert on the whole list or the empty state.
- The API is started with `AUTH_RATE_LIMIT=false` and `APP_DEBUG=true` (no throttling, mail only logged), after migrating the test database: its scan worker reads the `scans` table at boot, and Playwright starts it before `globalSetup`.
- The test API runs the scheduler and the scan worker, so a folder a spec creates gets its first scan within a minute. A spec that clicks Scan now accepts 409 `scans.in_progress` for the rare run where the schedule queued one first.
- Emailed links cannot be read back (only a hash is stored): plant a token with `dbHelper.createEmailToken(userId, purpose)`.
- Requires `TEST_DATABASE_PATH` (a SQLite file whose name ends in `_test.db`, relative to `../backend`) in `../backend/.env`. The helper opens the same file the API writes.

## Don’t
- Don’t call `fetch` or `api` from a page; go through `features/<module>/hooks.ts`.
- Don’t start `npm run dev` unless the user asks.
