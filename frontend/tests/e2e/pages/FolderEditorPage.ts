import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/** `/folders/new` and `/folders/:id/edit`. */
export class FolderEditorPage extends BasePage {
  get root(): Locator {
    return this.page.getByTestId('folder-editor-page')
  }

  get nameInput(): Locator {
    return this.page.getByTestId('folder-name-input')
  }

  get pathInput(): Locator {
    return this.page.getByTestId('folder-path-input')
  }

  get saveButton(): Locator {
    return this.page.getByTestId('folder-save-button')
  }

  get jpegQualityInput(): Locator {
    return this.page.getByTestId('folder-jpeg-quality-input')
  }

  async fill(name: string, path: string): Promise<void> {
    await expect(this.root).toBeVisible()
    await this.nameInput.fill(name)
    await this.pathInput.fill(path)
  }

  async openCompression(): Promise<void> {
    await this.page.getByTestId('folder-compression-toggle').click()
  }

  async chooseMode(mode: 'lossy' | 'lossless'): Promise<void> {
    await this.page.getByTestId(`folder-mode-option-${mode}`).click()
  }

  async delete(): Promise<void> {
    await this.page.getByTestId('folder-delete-button').click()
    await this.page.getByTestId('folder-delete-confirm').click()
  }
}
