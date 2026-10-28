import { test, expect } from '@playwright/test'
import { loginAs } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'

test.describe('Top bar', () => {
  test('goes home from the logo', async ({ page }) => {
    await loginAs(page, 'admin')
    const shell = new AppShell(page)

    await shell.goto('/profile')
    await page.getByTestId('app-logo').click()
    await expect(page.getByTestId('dashboard-page')).toBeVisible()
  })

  test('opens the nav from the menu button on a phone', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await loginAs(page, 'admin')
    const shell = new AppShell(page)

    await shell.goto('/')
    await expect(shell.navItem('storages')).toBeHidden()
    await shell.mobileMenuToggle.click()
    await shell.navItem('storages').click()
    await expect(page.getByTestId('storages-page')).toBeVisible()
    await expect(page.getByTestId('mobile-menu-drawer')).toBeHidden()
    await shell.openProfile()
  })
})
