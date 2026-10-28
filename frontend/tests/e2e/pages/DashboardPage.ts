import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

export class DashboardPage extends BasePage {
  get root(): Locator {
    return this.page.getByTestId('dashboard-page')
  }

  get empty(): Locator {
    return this.page.getByTestId('dashboard-empty')
  }

  async gotoDashboard(): Promise<void> {
    await this.goto('/')
    await expect(this.root).toBeVisible()
  }
}
