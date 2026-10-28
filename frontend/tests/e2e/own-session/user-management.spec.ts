import { test, expect } from '@playwright/test'
import { uniqueEmail } from '../helpers/db-helper'
import { dbHelper, loginAs } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'
import { LoginPage } from '../pages/LoginPage'
import { UserManagementPage } from '../pages/UserManagementPage'

test.describe('User management as a super admin', () => {
  test('creates a super admin from the user menu', async ({ page }) => {
    await loginAs(page, 'super-admin')
    const shell = new AppShell(page)
    const users = new UserManagementPage(page)
    const email = uniqueEmail('super')

    await shell.goto('/')
    await expect(shell.navItem('users')).toHaveCount(0)
    await shell.openUserMenu()
    await shell.userMenuItem('users').click()
    await expect(users.root('user')).toBeVisible()
    await users.tab('super-admin').click()
    await expect(users.root('super-admin')).toBeVisible()
    await expect(page).toHaveURL(/\/users\?role=super-admin$/)

    await users.create('E2E New Super Admin', email)
    await page.reload()
    await expect(users.root('super-admin')).toBeVisible()
    await users.rename(email, 'E2E Renamed Super Admin')
    await users.delete(email)
  })

  test('creates an admin who can then sign in', async ({ page }) => {
    await loginAs(page, 'super-admin')
    const shell = new AppShell(page)
    const users = new UserManagementPage(page)
    const email = uniqueEmail('admin')
    const password = 'admin-password-123'

    await users.open('user')
    await users.tab('admin').click()
    await expect(users.root('admin')).toBeVisible()

    await users.create('E2E New Admin', email)
    await users.setPassword(email, password)

    await shell.logoutViaUI()
    await new LoginPage(page).submit(email, password)
    await expect(shell.navItem('storages')).toBeVisible({ timeout: 15000 })
    await shell.openUserMenu()
    await expect(shell.userMenuItem('users')).toBeVisible()
  })

  test('cannot delete themselves', async ({ page }) => {
    const me = await loginAs(page, 'super-admin')
    const users = new UserManagementPage(page)

    await users.open('super-admin')
    await expect(users.row(me.email)).toBeVisible()
    await expect(users.row(me.email).locator('[data-testid^="edit-user-button-"]')).toBeVisible()
    await expect(users.row(me.email).locator('[data-testid^="delete-user-button-"]')).toHaveCount(0)
  })

  test('sees a taken email as an error on the email field', async ({ page }) => {
    await loginAs(page, 'super-admin')
    const existing = await dbHelper.createUser({ roles: ['admin'] })
    const users = new UserManagementPage(page)

    await users.open('admin')
    await page.getByTestId('add-user-button').click()
    await page.getByTestId('user-name-input').fill('E2E Duplicate')
    await page.getByTestId('user-email-input').fill(existing.email)
    await page.getByTestId('user-modal-submit-button').click()

    await expect(page.getByTestId('user-email-input')).toHaveAttribute('aria-invalid', 'true')
    await expect(page.getByTestId('user-name-input')).not.toHaveAttribute('aria-invalid', 'true')
    await expect(page.getByTestId('user-modal-submit-button')).toBeVisible()
    await expect(users.table.locator('tr[data-row-key]').filter({ hasText: 'E2E Duplicate' })).toHaveCount(0)
  })

  test('follows the old admin pages to their tabs', async ({ page }) => {
    await loginAs(page, 'super-admin')
    const users = new UserManagementPage(page)

    await users.goto('/admin/admins')
    await expect(users.root('admin')).toBeVisible()
    await users.goto('/admin/super-admins')
    await expect(users.root('super-admin')).toBeVisible()
  })
})

test.describe('User management as an admin', () => {
  test('manages plain users from the user menu, with no other tabs', async ({ page }) => {
    await loginAs(page, 'admin')
    const shell = new AppShell(page)
    const users = new UserManagementPage(page)
    const email = uniqueEmail('plain')

    await shell.goto('/')
    await shell.openUserMenu()
    await shell.userMenuItem('users').click()
    await expect(users.root('user')).toBeVisible()
    await expect(users.tab('admin')).toHaveCount(0)
    await expect(users.tab('super-admin')).toHaveCount(0)

    await users.create('E2E New User', email)
    await users.delete(email)
  })

  test('is refused on the admin and super admin lists', async ({ page }) => {
    await loginAs(page, 'admin')
    const users = new UserManagementPage(page)

    for (const path of ['/users?role=super-admin', '/users?role=admin', '/admin/admins']) {
      await users.goto(path)
      await expect(users.forbidden).toBeVisible()
      await expect(users.table).toHaveCount(0)
    }
  })
})

test.describe('User management as a plain user', () => {
  test('has no Users entry and is refused on every list', async ({ page }) => {
    await loginAs(page, 'user')
    const shell = new AppShell(page)
    const users = new UserManagementPage(page)

    await shell.goto('/')
    await expect(shell.navItem('dashboard')).toBeVisible()
    await shell.openUserMenu()
    await expect(shell.userMenuItem('users')).toHaveCount(0)

    for (const path of ['/users', '/users?role=admin', '/users?role=super-admin']) {
      await users.goto(path)
      await expect(users.forbidden).toBeVisible()
    }
  })
})
