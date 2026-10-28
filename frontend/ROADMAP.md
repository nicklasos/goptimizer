# Roadmap

What is worth adding to the skeleton next, in priority order. Each item names the project it can be ported from. Paths are relative to the directory that holds `gogo-front`.

## P1

| Item | Why | Port from |
|---|---|---|
| Smaller first load | Pages are lazy and React is cached separately, but the first load is still ~1.1 MB (about 350 kB gzipped), nearly all of it the Ant Design parts the shell and login form use | — |
| Page-error collector in E2E | Fails a test on console errors and uncaught exceptions | `smartcity-backoffice-front/tests/e2e/helpers/pageErrors.js` |

## P2

| Item | Why | Port from |
|---|---|---|
| Server-side search and filters | A filter row above a table whose values go into the query params, debounced | `smartcity-backoffice-front/src/hooks/useDebouncedCallback.js` |
| Theme tokens + dark mode | `app/theme.ts` is the single place; add an algorithm switch | `sytno/frontend/src/app/theme.ts` for a full token set |
| Registration page | The API can open `/auth/register`; there is no page for it | — |
| Role editing | Roles are fixed when an account is created; needs the matching gogo endpoint | — |

## P3

| Item | Why | Port from |
|---|---|---|
| Client error reporting | Posts front-end errors to the API; pairs with the gogo client-errors endpoint | `smartcity-backoffice-front/src/utils/reportClientError.js` |
| Audit log modal | Who changed what; pairs with gogo audit logs | `smartcity-backoffice-front/src/components/AuditLogModal.jsx` |

## Left out on purpose

- **Component tests**: a second test stack (vitest + testing-library) for little gain over `node:test` plus Playwright.
- **Generic CRUD hook factory**: hooks are about ten lines per entity and a factory hides the query keys. Revisit after three identical modules.
- **Per-module permissions**: roles are enough for a skeleton.
