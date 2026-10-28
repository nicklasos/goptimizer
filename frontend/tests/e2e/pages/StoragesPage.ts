import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

export class StoragesPage extends BasePage {
  get root(): Locator {
    return this.page.getByTestId('storages-page')
  }

  get table(): Locator {
    return this.page.getByTestId('storages-table')
  }

  get addButton(): Locator {
    return this.page.getByTestId('storages-add-button')
  }

  get forbidden(): Locator {
    return this.page.getByTestId('forbidden-result')
  }

  row(name: string): Locator {
    return this.table.locator('tr[data-row-key]').filter({ hasText: name })
  }

  async gotoList(): Promise<void> {
    await this.goto('/storages')
    await expect(this.root).toBeVisible()
  }

  async clickAdd(): Promise<void> {
    await this.addButton.click()
    await expect(this.page.getByTestId('storage-editor-page')).toBeVisible()
  }

  async edit(name: string): Promise<void> {
    await this.row(name).locator('[data-testid^="edit-storage-button-"]').click()
    await expect(this.page.getByTestId('storage-editor-page')).toBeVisible()
  }

  async browse(name: string): Promise<void> {
    await this.row(name).locator('[data-testid^="browse-storage-button-"]').click()
    await expect(this.page.getByTestId('storage-browser-page')).toBeVisible()
  }

  async delete(name: string): Promise<void> {
    await this.row(name).locator('[data-testid^="delete-storage-button-"]').click()
    await this.page.locator('[data-testid^="confirm-delete-storage-button-"]').click()
    await expect(this.row(name)).toHaveCount(0)
  }
}
