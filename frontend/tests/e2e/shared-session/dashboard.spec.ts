import { test, expect } from '@playwright/test'
import { DashboardPage } from '../pages/DashboardPage'

test.describe('Dashboard', () => {
  test('greets a plain user without showing any plan', async ({ page }) => {
    const dashboard = new DashboardPage(page)
    await dashboard.gotoDashboard()
    await expect(dashboard.noAccess).toBeVisible()
    await expect(dashboard.cards).toHaveCount(0)
  })
})
