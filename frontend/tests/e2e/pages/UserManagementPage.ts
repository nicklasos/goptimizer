import { expect, type Locator } from '@playwright/test'
import type { TestRole } from '../helpers/db-helper'
import { BasePage } from './BasePage'


export class UserManagementPage extends BasePage {
  root(role: TestRole): Locator {
    return this.page.getByTestId(`users-page-${role}`)
  }

  get table(): Locator {
    return this.page.getByTestId('users-table')
  }

  get forbidden(): Locator {
    return this.page.getByTestId('forbidden-result')
  }

  tab(role: TestRole): Locator {
    return this.page.getByTestId(`users-tab-${role}`)
  }

  row(email: string): Locator {
    return this.table.locator('tr[data-row-key]').filter({ hasText: email })
  }

  async open(role: TestRole): Promise<void> {
    await this.goto(role === 'user' ? '/users' : `/users?role=${role}`)
    await expect(this.root(role)).toBeVisible()
  }

  async create(name: string, email: string): Promise<void> {
    await this.page.getByTestId('add-user-button').click()
    await this.page.getByTestId('user-name-input').fill(name)
    await this.page.getByTestId('user-email-input').fill(email)
    await expect(this.page.getByTestId('user-password-input')).not.toHaveValue('')
    await this.page.getByTestId('user-modal-submit-button').click()
    await expect(this.row(email)).toBeVisible()
  }

  async rename(email: string, name: string): Promise<void> {
    await this.row(email).locator('[data-testid^="edit-user-button-"]').click()
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
