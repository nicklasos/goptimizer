import { test, expect } from '@playwright/test'
import { AppShell } from '../pages/AppShell'

test.describe('Unknown URLs', () => {
  test('show a not-found page inside the app', async ({ page }) => {
    const shell = new AppShell(page)

    await shell.goto('/no-such-page')
    await expect(page.getByTestId('page-not-found')).toBeVisible()
    await expect(page.getByTestId('app-header')).toBeVisible()
    await expect(page).toHaveURL(/no-such-page/)

    await page.getByTestId('not-found-home-button').click()
    await expect(page).toHaveURL(/\/$/)
  })

  test('nested unknown paths are not found either', async ({ page }) => {
    await new AppShell(page).goto('/users/12/no-such-tab')
    await expect(page.getByTestId('page-not-found')).toBeVisible()
  })
})
