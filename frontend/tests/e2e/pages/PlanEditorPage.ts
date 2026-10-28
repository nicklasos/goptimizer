import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

export type SourceKind = 'postgres' | 'mysql' | 'sqlite' | 'files'
export type Weekday = 'mon' | 'tue' | 'wed' | 'thu' | 'fri' | 'sat' | 'sun'

export class PlanEditorPage extends BasePage {
  get root(): Locator {
    return this.page.getByTestId('plan-editor-page')
  }

  get nameInput(): Locator {
    return this.page.getByTestId('plan-name-input')
  }

  get urlInput(): Locator {
    return this.page.getByTestId('plan-source-url-input')
  }

  get hostInput(): Locator {
    return this.page.getByTestId('plan-db-host-input')
  }

  get portInput(): Locator {
    return this.page.getByTestId('plan-db-port-input')
  }

  get databaseInput(): Locator {
    return this.page.getByTestId('plan-db-database-input')
  }

  get userInput(): Locator {
    return this.page.getByTestId('plan-db-user-input')
  }

  get passwordInput(): Locator {
    return this.page.getByTestId('plan-db-password-input')
  }

  get sqlitePathInput(): Locator {
    return this.page.getByTestId('plan-sqlite-path-input')
  }

  get testButton(): Locator {
    return this.page.getByTestId('plan-test-button')
  }

  get testPassed(): Locator {
    return this.page.getByTestId('plan-test-ok')
  }

  get testFailed(): Locator {
    return this.page.getByTestId('plan-test-failed')
  }

  get toolVersion(): Locator {
    return this.page.getByTestId('plan-test-tool-version')
  }

  get serverVersion(): Locator {
    return this.page.getByTestId('plan-test-server-version')
  }

  get scheduleRows(): Locator {
    return this.page.locator('[data-testid^="plan-schedule-row-"]')
  }

  get addScheduleButton(): Locator {
    return this.page.getByTestId('plan-schedule-add')
  }

  get nextRun(): Locator {
    return this.page.getByTestId('plan-next-run')
  }

  get storagesSelect(): Locator {
    return this.page.getByTestId('plan-storages-select')
  }

  get retentionSummary(): Locator {
    return this.page.getByTestId('plan-retention-summary')
  }

  get saveButton(): Locator {
    return this.page.getByTestId('plan-save-button')
  }

  get error(): Locator {
    return this.page.getByTestId('plan-editor-error')
  }

  async chooseKind(kind: SourceKind): Promise<void> {
    await this.page.getByTestId(`plan-kind-option-${kind}`).click()
  }

  async testConnection(): Promise<void> {
    await this.testButton.click()
    await expect(this.testPassed.or(this.testFailed)).toBeVisible({ timeout: 20000 })
  }

  /** The picker's text box: the test id may sit on it or on its wrapper. */
  timeInput(row: number): Locator {
    const id = `plan-schedule-time-${row}`
    return this.page.locator(`input[data-testid="${id}"], [data-testid="${id}"] input`)
  }

  async setTime(row: number, time: string): Promise<void> {
    const input = this.timeInput(row)
    await input.click()
    await input.fill(time)
    await input.press('Tab')
    await expect(input).toHaveValue(time)
  }

  async toggleDay(row: number, day: Weekday): Promise<void> {
    await this.page.getByTestId(`plan-schedule-${row}-day-${day}`).click()
  }

  async removeSchedule(row: number): Promise<void> {
    await this.page.getByTestId(`plan-schedule-remove-${row}`).click()
  }

  async chooseStorage(name: string): Promise<void> {
    await this.storagesSelect.click()
    await this.page.keyboard.type(name)
    await this.page.keyboard.press('Enter')
    await this.page.keyboard.press('Escape')
  }

  async setRetention(days: number, weeks: number, months: number): Promise<void> {
    await this.page.getByTestId('plan-max-age-input').fill(String(days))
    await this.page.getByTestId('plan-keep-weekly-input').fill(String(weeks))
    await this.page.getByTestId('plan-keep-monthly-input').fill(String(months))
  }

  /** Saving lands on the plan's page. */
  async save(): Promise<void> {
    await this.saveButton.click()
    await expect(this.page.getByTestId('plan-page')).toBeVisible({ timeout: 15000 })
  }

  async delete(): Promise<void> {
    await this.page.getByTestId('plan-editor-delete-button').click()
    await this.page.getByTestId('plan-editor-delete-confirm').click()
  }
}
