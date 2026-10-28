import { test, expect } from '@playwright/test'
import dbHelper from '../helpers/db-helper'
import { AppShell } from '../pages/AppShell'
import { LoginPage } from '../pages/LoginPage'

test.describe('Password reset', () => {
  test('asks for a link without revealing whether the email exists', async ({ page }) => {
    const loginPage = new LoginPage(page)
    await loginPage.goto()
    await page.getByTestId('forgot-password-link').click()

    await page.getByTestId('forgot-password-email-input').fill('nobody-here@example.com')
    await page.getByTestId('forgot-password-submit-button').click()
    await expect(page.getByTestId('forgot-password-sent')).toBeVisible()

    await page.getByTestId('back-to-login-link').click()
    await expect(loginPage.form).toBeVisible()
  })

  test('sets a new password from the emailed link', async ({ page }) => {
    const user = await dbHelper.createUser()
    const token = await dbHelper.createEmailToken(user.id, 'password_reset')
    const loginPage = new LoginPage(page)
    const newPassword = 'reset-password-123'

    await page.goto(`/reset-password?token=${token}`)
    await page.getByTestId('reset-password-input').fill(newPassword)
    await page.getByTestId('reset-password-confirm-input').fill(newPassword)
    await page.getByTestId('reset-password-submit-button').click()

    await expect(loginPage.form).toBeVisible()
    await loginPage.submit(user.email, user.password)
    await expect(loginPage.errorAlert).toBeVisible()
    await loginPage.submit(user.email, newPassword)
    await expect(new AppShell(page).mainContent).toBeVisible({ timeout: 15000 })
  })

  test('refuses a link that was already used', async ({ page }) => {
    const user = await dbHelper.createUser()
    const token = await dbHelper.createEmailToken(user.id, 'password_reset')

    for (const expectError of [false, true]) {
      await page.goto(`/reset-password?token=${token}`)
      await page.getByTestId('reset-password-input').fill('reset-password-123')
      await page.getByTestId('reset-password-confirm-input').fill('reset-password-123')
      await page.getByTestId('reset-password-submit-button').click()
      if (expectError) {
        await expect(page.getByTestId('reset-password-error')).toBeVisible()
        await expect(page.getByTestId('request-new-link')).toBeVisible()
      } else {
        await expect(page.getByTestId('login-form')).toBeVisible()
      }
    }
  })

  test('refuses an expired link and a missing token', async ({ page }) => {
    const user = await dbHelper.createUser()
    const expired = await dbHelper.createEmailToken(user.id, 'password_reset', { expired: true })

    await page.goto(`/reset-password?token=${expired}`)
    await page.getByTestId('reset-password-input').fill('reset-password-123')
    await page.getByTestId('reset-password-confirm-input').fill('reset-password-123')
    await page.getByTestId('reset-password-submit-button').click()
    await expect(page.getByTestId('reset-password-error')).toBeVisible()

    await page.goto('/reset-password')
    await expect(page.getByTestId('reset-password-invalid')).toBeVisible()
  })

  test('tells a throttled user how long to wait', async ({ page }) => {
    await page.route('**/api/v1/auth/login', (route) =>
      route.fulfill({
        status: 429,
        contentType: 'application/json',
        headers: { 'Retry-After': '300' },
        body: JSON.stringify({
          error_key: 'auth.too_many_requests',
          message: 'Too many attempts. Please try again later.',
          status: 429,
          details: { retry_after_seconds: 300 },
        }),
      })
    )

    const loginPage = new LoginPage(page)
    await loginPage.goto()
    await loginPage.submit('someone@example.com', 'whatever-password')

    await expect(loginPage.errorAlert).toHaveAttribute('data-error-type', 'rate_limit')
    await expect(loginPage.errorAlert).toContainText('5')
    await expect(loginPage.submitButton).toBeEnabled()
  })
})
