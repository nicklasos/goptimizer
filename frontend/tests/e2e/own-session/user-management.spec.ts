import { test, expect } from '@playwright/test'
import { uniqueEmail } from '../helpers/db-helper'
import { dbHelper, loginAs } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'
import { LoginPage } from '../pages/LoginPage'
import { UserManagementPage } from '../pages/UserManagementPage'

test.describe('User management', () => {
  test('an admin creates an admin who can then manage users', async ({ page }) => {
    await loginAs(page, 'admin')
    const shell = new AppShell(page)
    const users = new UserManagementPage(page)
    const email = uniqueEmail('admin')
    const password = 'admin-password-123'

    await shell.goto('/')
    await expect(shell.navItem('users')).toHaveCount(0)
    await shell.openUserMenu()
    await shell.userMenuItem('users').click()
    await expect(users.root).toBeVisible()

    await users.create('E2E New Admin', email, { admin: true })
    await expect(users.row(email).getByTestId('user-admin-tag')).toBeVisible()
    await users.setPassword(email, password)

    await shell.logoutViaUI()
    await new LoginPage(page).submit(email, password)
    await expect(shell.userMenuTrigger).toBeVisible({ timeout: 15000 })
    await shell.openUserMenu()
    await expect(shell.userMenuItem('users')).toBeVisible()
  })

  test('an admin creates, renames and deletes a plain user', async ({ page }) => {
    await loginAs(page, 'admin')
    const users = new UserManagementPage(page)
    const email = uniqueEmail('plain')

    await users.open()
    await users.create('E2E New User', email)
    await expect(users.row(email).getByTestId('user-admin-tag')).toHaveCount(0)
    await page.reload()
    await users.rename(email, 'E2E Renamed User')
    await users.delete(email)
  })

  test('an admin cannot delete themselves or remove their own admin flag', async ({ page }) => {
    const me = await loginAs(page, 'admin')
    const other = await dbHelper.createUser({ isAdmin: true })
    const users = new UserManagementPage(page)

    await users.open()
    await expect(users.row(me.email).locator('[data-testid^="delete-user-button-"]')).toHaveCount(0)
    await users.edit(me.email)
    await expect(users.adminSwitch).toBeDisabled()
    await page.getByTestId('user-modal-cancel-button').click()

    await expect(users.row(other.email).locator('[data-testid^="delete-user-button-"]')).toBeVisible()
    await users.edit(other.email)
    await expect(users.adminSwitch).toBeEnabled()
  })

  test('sees a taken email as an error on the email field', async ({ page }) => {
    await loginAs(page, 'admin')
    const existing = await dbHelper.createUser()
    const users = new UserManagementPage(page)

    await users.open()
    await page.getByTestId('add-user-button').click()
    await page.getByTestId('user-name-input').fill('E2E Duplicate')
    await page.getByTestId('user-email-input').fill(existing.email)
    await page.getByTestId('user-modal-submit-button').click()

    await expect(page.getByTestId('user-email-input')).toHaveAttribute('aria-invalid', 'true')
    await expect(page.getByTestId('user-name-input')).not.toHaveAttribute('aria-invalid', 'true')
    await expect(page.getByTestId('user-modal-submit-button')).toBeVisible()
    await expect(users.table.locator('tr[data-row-key]').filter({ hasText: 'E2E Duplicate' })).toHaveCount(0)
  })
})
