import { expect, type Locator, type Page } from '@playwright/test'

export class BasePage {
  constructor(readonly page: Page) {}

  get mainContent(): Locator {
    return this.page.getByTestId('main-content')
  }

  get loginEmailInput(): Locator {
    return this.page.getByTestId('login-email-input')
  }

  async goto(path: string): Promise<void> {
    await this.page.goto(path)
    await this.page.waitForLoadState('domcontentloaded')
    await this.expectLoaded()
  }

  async expectLoaded(): Promise<void> {
    await expect(this.mainContent).toBeVisible({ timeout: 15000 })
  }
}
