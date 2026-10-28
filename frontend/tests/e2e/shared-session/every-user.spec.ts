import { test, expect } from '@playwright/test'
import { AppShell } from '../pages/AppShell'

test.describe('A plain user', () => {
  test('uses folders and settings from the top bar', async ({ page }) => {
    const shell = new AppShell(page)
    await shell.goto('/')
    await expect(page.getByTestId('dashboard-page')).toBeVisible()
    await shell.navItem('settings').click()
    await expect(page.getByTestId('settings-page')).toBeVisible()
  })

  test('has no Users entry and is refused on /users', async ({ page }) => {
    const shell = new AppShell(page)
    await shell.goto('/')
    await shell.openUserMenu()
    await expect(shell.userMenuItem('users')).toHaveCount(0)

    await shell.goto('/users')
    await expect(page.getByTestId('forbidden-result')).toBeVisible()
    await expect(page.getByTestId('users-table')).toHaveCount(0)
  })
})
