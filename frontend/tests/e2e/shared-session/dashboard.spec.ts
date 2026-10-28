import { test, expect } from '@playwright/test'
import { DashboardPage } from '../pages/DashboardPage'

test.describe('Dashboard', () => {
  test('shows the empty state until folders arrive', async ({ page }) => {
    const dashboard = new DashboardPage(page)
    await dashboard.gotoDashboard()
    await expect(dashboard.empty).toBeVisible()
  })
})
