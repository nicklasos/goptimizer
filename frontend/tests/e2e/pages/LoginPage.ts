import { expect, type Locator, type Page } from '@playwright/test'

export class LoginPage {
  constructor(readonly page: Page) {}

  get form(): Locator {
    return this.page.getByTestId('login-form')
  }

  get emailInput(): Locator {
    return this.page.getByTestId('login-email-input')
  }

  get passwordInput(): Locator {
    return this.page.getByTestId('login-password-input')
  }

  get submitButton(): Locator {
    return this.page.getByTestId('login-button')
  }

  get errorAlert(): Locator {
    return this.page.getByTestId('login-error-alert')
  }

  get invalidFields(): Locator {
    return this.form.locator('[aria-invalid="true"]')
  }

  async goto(): Promise<void> {
    await this.page.goto('/')
  }

  async submit(email: string, password: string): Promise<void> {
    await this.emailInput.fill(email)
    await this.passwordInput.fill(password)
    await this.submitButton.click()
  }

  async login(email: string, password: string): Promise<void> {
    await this.goto()
    await this.submit(email, password)
    await expect(this.page.getByTestId('main-content')).toBeVisible({ timeout: 15000 })
    await expect(this.page).toHaveURL('/')
  }
}
