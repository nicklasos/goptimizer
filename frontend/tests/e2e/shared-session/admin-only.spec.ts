import { test, expect } from '@playwright/test'
import { AppShell } from '../pages/AppShell'

test.describe('Admin pages', () => {
  for (const path of ['/', '/folders/new', '/settings']) {
    test(`are forbidden to a plain user: ${path}`, async ({ page }) => {
      const shell = new AppShell(page)
      await shell.goto(path)
      await expect(page.getByTestId('forbidden-result')).toBeVisible()
      await expect(shell.navItem('settings')).toHaveCount(0)
    })
  }
})
