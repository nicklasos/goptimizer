import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

export class DashboardPage extends BasePage {
  get root(): Locator {
    return this.page.getByTestId('dashboard-page')
  }

  get noAccess(): Locator {
    return this.page.getByTestId('dashboard-no-access')
  }

  get cards(): Locator {
    return this.page.locator('[data-testid^="plan-card-"][data-health]')
  }

  get addButton(): Locator {
    return this.page.getByTestId('plans-add-button')
  }

  get forbidden(): Locator {
    return this.page.getByTestId('forbidden-result')
  }

  /** A plan's card; its `data-health` is ok, failed, overdue, running, paused or never. */
  card(planId: number): Locator {
    return this.page.getByTestId(`plan-card-${planId}`)
  }

  health(planId: number): Locator {
    return this.page.getByTestId(`plan-health-${planId}`)
  }

  /** A day of the plan's 30-day strip (YYYY-MM-DD, UTC); its `data-kind` is succeeded, failed or none. */
  day(planId: number, date: string): Locator {
    return this.page.getByTestId(`plan-day-${planId}-${date}`)
  }

  async runNow(planId: number): Promise<void> {
    await this.page.getByTestId(`run-plan-button-${planId}`).click()
  }

  async open(planId: number): Promise<void> {
    await this.page.getByTestId(`plan-card-name-${planId}`).click()
    await expect(this.page.getByTestId('plan-page')).toBeVisible()
  }

  async gotoDashboard(): Promise<void> {
    await this.goto('/')
    await expect(this.root).toBeVisible()
  }

  async clickAdd(): Promise<void> {
    await this.addButton.click()
    await expect(this.page.getByTestId('plan-editor-page')).toBeVisible()
  }
}
