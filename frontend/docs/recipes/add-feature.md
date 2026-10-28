# Add a feature to the admin panel

A feature is the screens for one API resource. The steps add `orders`; copy `src/features/users`
and rename as you go. The API must exist first: see [full-stack-feature.md](full-stack-feature.md).

## 1. `src/features/orders/`

| File | Contents |
|---|---|
| `types.ts` | Short names for the generated types: `export type Order = InternalOrdersOrderResponse`. Hand-written types only for things the API does not describe, such as form values. |
| `api.ts` | `export const ordersApi = { list, get, create, update, remove }` built on `api` from `@/api/client`. `api.getPage<T>()` for paginated lists, `api.get/post/put/del<T>()` for the rest. |
| `hooks.ts` | `useOrders(params)`, `useOrder(id)`, `useSaveOrder()` (create or update by `id`), `useDeleteOrder()`. Mutations invalidate `qk.orders.all`. |
| `policy.ts` | `canUpdateOrder(user, order)` and friends, mirroring the API's `policy.go`. |
| `pages/*.tsx` | The screens. Default exports, because the router loads them lazily. |

Then add the query keys to `src/api/queryKeys.ts`:

```ts
orders: {
  all: ['orders'] as const,
  list: (params: PageParams) => ['orders', 'list', params] as const,
  detail: (id: ID) => ['orders', 'detail', id] as const,
},
```

## 2. Pages

Page skeleton: `PageStack` > `Card` > `PageHeader` (title, `mainAction`) + content.

- **A list**: `usePageParams()` + the list hook + `useTablePagination(paging, query.data)`, rendered with
  `ResponsiveTable` (give it `columns` for desktop and `cardTitle` / `renderCard` for phones). Row buttons through `RowActions`.
- **A short form**: `ModalForm` on the list page (see `features/users`).
- **A long form**: its own page: `PageLoading` / `PageError` while loading,
  `useUnsavedChangesGuard(dirty)`, and leave through `useGuardedNavigate()`.
- **Saving**: in the mutation's `onError`, `applyFormErrors(form, t, error, FIELDS)` puts server validation
  errors under their fields and returns the message for anything else. Mark that mutation `meta: { silent: true }`
  so the global toast does not repeat it.
- Pages call hooks, never `api` or `fetch`.
- Style with Ant Design components and `theme.useToken()`; no literal colours.

## 3. Wire it in

1. `src/app/router.tsx`: `const OrdersPage = lazy(() => import('@/features/orders/pages/OrdersPage'))` and a `<Route>`.
   Wrap it in `<RequireRole roles={[...]}>` when only some roles may open it.
2. `src/layout/nav.tsx`: an entry in `TOP_NAV` (the top bar) or `USER_MENU` (under the user's name, for account and administration pages) with `roles` and a `testId`.
3. Both `src/locales/en/translation.json` and `src/locales/uk/translation.json`: `navigation.orders`, an `orders.*` section,
   and `errors.<key>` for every error key the API can return for this resource. A unit test fails when the two files differ.

## 4. Tests

- `policy.ts` gets cases in `src/features/policies.test.ts`.
- Pure helpers get a `*.test.ts` next to them.
- One Playwright spec for the main flow: see [add-e2e-test.md](add-e2e-test.md).

## 5. Finish

```bash
make test
```

## Check

- `make typecheck`, `make lint` and `make test` pass.
- The list survives a reload on page 2 (the page is in the URL).
- Submitting an invalid form shows the error under the field, not only as a toast.
- At phone width the table turns into cards and the primary action moves to the bottom bar.
