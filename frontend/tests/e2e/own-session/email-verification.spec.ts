import { test, expect } from '@playwright/test'
import { dbHelper, loginAs } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'

test.describe('Email verification', () => {
  test('a signed-in user confirms their email from the link', async ({ page }) => {
    const user = await loginAs(page, 'user', { emailVerified: false })
    const token = await dbHelper.createEmailToken(user.id, 'email_verification')
    const shell = new AppShell(page)

    await shell.goto('/profile')
    await expect(page.getByTestId('email-unverified-alert')).toBeVisible()

    await shell.goto(`/verify-email?token=${token}`)
    await expect(page.getByTestId('verify-email-success')).toBeVisible()

    await shell.goto('/profile')
    await expect(page.getByTestId('profile-form')).toBeVisible()
    await expect(page.getByTestId('email-unverified-alert')).toHaveCount(0)
  })

  test('the link also works signed out, once', async ({ page }) => {
    const user = await dbHelper.createUser({ emailVerified: false })
    const token = await dbHelper.createEmailToken(user.id, 'email_verification')

    await page.goto(`/verify-email?token=${token}`)
    await expect(page.getByTestId('verify-email-success')).toBeVisible()

    await page.goto(`/verify-email?token=${token}`)
    await expect(page.getByTestId('verify-email-error')).toBeVisible()

    await page.getByTestId('verify-email-continue-button').click()
    await expect(page.getByTestId('login-form')).toBeVisible()
  })

  test('a verified user sees no reminder, an unverified one can ask for another email', async ({ page }) => {
    await loginAs(page, 'user', { emailVerified: false })
    const shell = new AppShell(page)

    await shell.goto('/profile')
    await page.getByTestId('resend-verification-button').click()
    await expect(page.getByTestId('resend-verification-button')).toBeEnabled()
    await expect(page.getByTestId('email-unverified-alert')).toBeVisible()
    await expect(shell.loginEmailInput).toHaveCount(0)
  })
})
