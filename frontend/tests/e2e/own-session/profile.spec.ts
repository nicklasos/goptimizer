import { test, expect } from '@playwright/test'
import { loginAs } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'
import { LoginPage } from '../pages/LoginPage'

test.describe('Profile', () => {
  test('updates the name', async ({ page }) => {
    await loginAs(page, 'user')
    const shell = new AppShell(page)

    await shell.goto('/')
    await shell.openProfile()
    await page.getByTestId('profile-name-input').fill('Renamed E2E User')
    await page.getByTestId('profile-update-button').click()

    await expect(page.getByTestId('user-menu-name')).toHaveText('Renamed E2E User')
    await page.reload()
    await expect(page.getByTestId('profile-name-input')).toHaveValue('Renamed E2E User')
  })

  test('changes the password', async ({ page }) => {
    const user = await loginAs(page, 'user')
    const shell = new AppShell(page)
    const newPassword = 'new-password-123'

    await shell.goto('/profile')
    await page.getByTestId('profile-current-password-input').fill(user.password)
    await page.getByTestId('profile-new-password-input').fill(newPassword)
    await page.getByTestId('profile-confirm-password-input').fill(newPassword)
    await page.getByTestId('profile-change-password-button').click()
    await expect(page.getByTestId('profile-current-password-input')).toHaveValue('')

    await shell.logoutViaUI()
    const loginPage = new LoginPage(page)
    await loginPage.submit(user.email, user.password)
    await expect(loginPage.errorAlert).toBeVisible()
    await loginPage.submit(user.email, newPassword)
    await expect(shell.mainContent).toBeVisible({ timeout: 15000 })
  })

  test('keeps the session when the current password is wrong', async ({ page }) => {
    await loginAs(page, 'user')
    const shell = new AppShell(page)

    await shell.goto('/profile')
    await page.getByTestId('profile-current-password-input').fill('not-the-password')
    await page.getByTestId('profile-new-password-input').fill('new-password-123')
    await page.getByTestId('profile-confirm-password-input').fill('new-password-123')
    await page.getByTestId('profile-change-password-button').click()

    await expect(page.getByTestId('profile-current-password-input')).toHaveValue('not-the-password')
    await expect(page.getByTestId('profile-page')).toBeVisible()
    await expect(shell.loginEmailInput).toHaveCount(0)
  })

  test('switches the panel to Ukrainian and back, across a reload', async ({ page }) => {
    await loginAs(page, 'user')
    const shell = new AppShell(page)
    const html = page.locator('html')

    await shell.goto('/')
    await shell.openProfile()
    await expect(html).toHaveAttribute('lang', 'en')
    await page.getByTestId('language-option-uk').click()
    await expect(html).toHaveAttribute('lang', 'uk')

    await page.reload()
    await expect(page.getByTestId('profile-page')).toBeVisible()
    await expect(html).toHaveAttribute('lang', 'uk')

    await page.getByTestId('language-option-en').click()
    await expect(html).toHaveAttribute('lang', 'en')
    await page.reload()
    await expect(page.getByTestId('profile-page')).toBeVisible()
    await expect(html).toHaveAttribute('lang', 'en')
  })
})
