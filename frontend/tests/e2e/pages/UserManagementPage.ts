import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

export class UserManagementPage extends BasePage {
  get root(): Locator {
    return this.page.getByTestId('users-page')
  }

  get table(): Locator {
    return this.page.getByTestId('users-table')
  }

  get adminSwitch(): Locator {
    return this.page.getByTestId('user-admin-switch')
  }

  row(email: string): Locator {
    return this.table.locator('tr[data-row-key]').filter({ hasText: email })
  }

  async open(): Promise<void> {
    await this.goto('/users')
    await expect(this.root).toBeVisible()
  }

  async create(name: string, email: string, { admin = false } = {}): Promise<void> {
    await this.page.getByTestId('add-user-button').click()
    await this.page.getByTestId('user-name-input').fill(name)
    await this.page.getByTestId('user-email-input').fill(email)
    await expect(this.page.getByTestId('user-password-input')).not.toHaveValue('')
    if (admin) await this.adminSwitch.click()
    await this.page.getByTestId('user-modal-submit-button').click()
    await expect(this.row(email)).toBeVisible()
  }

  async edit(email: string): Promise<void> {
    await this.row(email).locator('[data-testid^="edit-user-button-"]').click()
    await expect(this.page.getByTestId('user-name-input')).toBeVisible()
  }

  async rename(email: string, name: string): Promise<void> {
    await this.edit(email)
    await this.page.getByTestId('user-name-input').fill(name)
    await this.page.getByTestId('user-modal-submit-button').click()
    await expect(this.row(email)).toContainText(name)
  }

  async setPassword(email: string, password: string): Promise<void> {
    await this.row(email).locator('[data-testid^="set-password-user-button-"]').click()
    await this.page.getByTestId('user-new-password-input').fill(password)
    await this.page.getByTestId('password-modal-submit-button').click()
    await expect(this.page.getByTestId('user-new-password-input')).toBeHidden()
  }

  async delete(email: string): Promise<void> {
    await this.row(email).locator('[data-testid^="delete-user-button-"]').click()
    await this.page.locator('[data-testid^="confirm-delete-user-button-"]').click()
    await expect(this.row(email)).toHaveCount(0)
  }
}
