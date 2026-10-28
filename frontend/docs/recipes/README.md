# Recipes

Step-by-step procedures for the changes this codebase sees most often. Each one ends with
how to check the result. They are written for coding agents and work as checklists for people.

| Recipe | Use it when |
|---|---|
| [full-stack-feature.md](full-stack-feature.md) | A feature needs work in gogo (the API) and here |
| [add-feature.md](add-feature.md) | The API exists and the admin panel needs screens for it |
| [add-e2e-test.md](add-e2e-test.md) | A flow needs a Playwright test |

`src/features/users` is the reference feature (paginated table, modal forms, policy). When a recipe
and the code disagree, the code wins, and the recipe should be fixed in the same change.

Rules that apply to every recipe:

- No narrating comments. A comment says why, not what.
- Every interactive element gets a `data-testid` (`<entity>-<element>[-<id>]`).
- Finish with `make test`; fix what fails before calling the work done.
- Do not start the dev server (`npm run dev`). The owner runs it at http://localhost:5747.
