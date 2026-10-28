import { test, expect } from '@playwright/test'
import dbHelper from '../helpers/db-helper'
import { AUTH_STORAGE_KEY } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'
import { LoginPage } from '../pages/LoginPage'

test.describe('Authentication', () => {
  let loginPage: LoginPage
  let appShell: AppShell

  test.beforeEach(async ({ page }) => {
    loginPage = new LoginPage(page)
    appShell = new AppShell(page)
  })

  test('should display login form', async () => {
    await loginPage.goto()
    await expect(loginPage.form).toBeVisible()
    await expect(loginPage.emailInput).toBeVisible()
    await expect(loginPage.passwordInput).toBeVisible()
    await expect(loginPage.submitButton).toBeVisible()
  })

  test('should show validation errors for empty fields', async () => {
    await loginPage.goto()
    await loginPage.submitButton.click()
    await expect(loginPage.invalidFields).toHaveCount(2)
  })

  test('should show error for invalid credentials', async () => {
    await loginPage.goto()
    await loginPage.submit('invalid@example.com', 'wrongpassword')
    await expect(loginPage.errorAlert).toBeVisible({ timeout: 10000 })
  })

  test('should login successfully with valid credentials', async ({ page }) => {
    const user = await dbHelper.createUser()
    await loginPage.login(user.email, user.password)
    await expect(page.getByTestId('dashboard-page')).toBeVisible()
  })

  test('should persist login state after page refresh', async ({ page }) => {
    const user = await dbHelper.createUser()
    await loginPage.login(user.email, user.password)
    await page.reload()
    await expect(appShell.mainContent).toBeVisible({ timeout: 15000 })
    await expect(page.getByTestId('dashboard-page')).toBeVisible()
  })

  test('should keep the session when the access token is no longer valid', async ({ page }) => {
    const user = await dbHelper.createUser()
    await loginPage.login(user.email, user.password)

    await page.evaluate((key) => {
      const stored = JSON.parse(localStorage.getItem(key) ?? '{}')
      stored.state.token = 'not-a-valid-token'
      localStorage.setItem(key, JSON.stringify(stored))
    }, AUTH_STORAGE_KEY)

    await page.goto('/profile')
    await expect(page.getByTestId('profile-email-input')).toHaveValue(user.email, { timeout: 15000 })
  })

  test('should show login when accessing routes without authentication', async ({ page }) => {
    await page.goto('/profile')
    await expect(loginPage.emailInput).toBeVisible()
  })

  test('should logout successfully', async () => {
    const user = await dbHelper.createUser()
    await loginPage.login(user.email, user.password)
    await appShell.logoutViaUI()
    await expect(loginPage.form).toBeVisible()
  })

  test('should prevent access after logout', async ({ page }) => {
    const user = await dbHelper.createUser()
    await loginPage.login(user.email, user.password)
    await appShell.logoutViaUI()
    await page.goto('/profile')
    await expect(loginPage.emailInput).toBeVisible()
  })
})
