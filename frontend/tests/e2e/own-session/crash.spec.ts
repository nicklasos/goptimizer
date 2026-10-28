import { test, expect } from '@playwright/test'
import { loginAs } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'
import { StoragesPage } from '../pages/StoragesPage'

test.describe('A page that crashes while rendering', () => {
  test('is contained: the menu keeps working and retrying recovers', async ({ page }) => {
    await loginAs(page, 'admin')
    const shell = new AppShell(page)
    const list = new StoragesPage(page)
    let broken = true

    // A row the page cannot render: the list endpoint answers with a null item.
    await page.route('**/api/v1/storages?*', (route) =>
      broken
        ? route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ data: [null], pagination: { total: 1, current_page: 1, last_page: 1, per_page: 20 } }),
          })
        : route.fallback()
    )

    await shell.goto('/storages')
    await expect(page.getByTestId('crash-screen')).toBeVisible()
    await expect(shell.mainMenu).toBeVisible()

    await shell.navItem('dashboard').click()
    await expect(page.getByTestId('dashboard-page')).toBeVisible()
    await expect(page.getByTestId('crash-screen')).toHaveCount(0)

    await shell.navItem('storages').click()
    await expect(page.getByTestId('crash-screen')).toBeVisible()

    broken = false
    await page.reload()
    await expect(list.root).toBeVisible()
    await expect(page.getByTestId('crash-screen')).toHaveCount(0)
  })
})
