# Build a feature across the API and the admin panel

The two repositories sit side by side: `../backend` is the API, this one is the UI. The contract
between them is the OpenAPI file gogo generates, which becomes `src/api/schema.ts` here.
Work in this order, so each step has something real to build on.

## 1. API first (in `../backend`)

Follow `../backend/docs/recipes/add-module.md` (a new resource) or `add-endpoint.md` (one more route).
That covers the migration, queries, policy, service, handlers, factory and tests, and ends with:

```bash
make swagger
make test
```

Do not continue until `make test` passes there.

## 2. Bring the contract over (here)

```bash
make api-types
```

`src/api/schema.ts` now has the new request and response types. Never edit that file.
If a field you expected is optional, missing or `string` instead of a union, fix the Go struct
(`validate:"optional"`, `enums:"a,b"`) and regenerate, rather than working around it here.

## 3. The UI

Follow [add-feature.md](add-feature.md).

## 4. Things that span both sides

| On the API | Here |
|---|---|
| A new error key in `internal/errs/keys.go` | A translation under `errors.<key>` in **both** locale files |
| A rule in `internal/<module>/policy.go` | The same rule in `src/features/<module>/policy.ts`, with the same test cases |
| A new role | `src/auth/roles.ts`, `enums.role.<role>` in the locales, and the menus in `src/layout/nav.tsx` |
| A new enum value | The generated union picks it up; add `enums.<name>.<value>` to the locales |
| A factory in `internal/factory` | A matching method in `tests/e2e/helpers/db-helper.ts` if E2E tests need the data |

## 5. Finish

```bash
make test          # here: typecheck, lint, unit, Playwright against a real gogo
```

The E2E run starts its own API from `../backend` on the test database, so it tests the two halves together.

## Check

- `make test` passes in both repositories.
- `make api-types-check` reports no difference (CI runs it).
- The new screens behave correctly for a user who is not allowed: the control is hidden, and opening the URL directly shows the forbidden page.
