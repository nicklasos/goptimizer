import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

export type StorageKind = 'local' | 's3' | 'gcs'

export class StorageEditorPage extends BasePage {
  get root(): Locator {
    return this.page.getByTestId('storage-editor-page')
  }

  get backButton(): Locator {
    return this.page.getByTestId('storage-editor-back-button')
  }

  get nameInput(): Locator {
    return this.page.getByTestId('storage-name-input')
  }

  get pathInput(): Locator {
    return this.page.getByTestId('storage-local-path-input')
  }

  get s3BucketInput(): Locator {
    return this.page.getByTestId('storage-s3-bucket-input')
  }

  get s3SecretKeyInput(): Locator {
    return this.page.getByTestId('storage-s3-secret-key-input')
  }

  get gcsCredentialsInput(): Locator {
    return this.page.getByTestId('storage-gcs-credentials-input')
  }

  get testButton(): Locator {
    return this.page.getByTestId('storage-test-button')
  }

  get testPassed(): Locator {
    return this.page.getByTestId('storage-test-ok')
  }

  get testFailed(): Locator {
    return this.page.getByTestId('storage-test-failed')
  }

  get saveButton(): Locator {
    return this.page.getByTestId('storage-save-button')
  }

  async chooseKind(kind: StorageKind): Promise<void> {
    await this.page.getByTestId(`storage-kind-option-${kind}`).click()
  }

  async testConnection(): Promise<void> {
    await this.testButton.click()
    await expect(this.testPassed.or(this.testFailed)).toBeVisible({ timeout: 15000 })
  }

  async save(): Promise<void> {
    await this.saveButton.click()
    await expect(this.page.getByTestId('storages-page')).toBeVisible({ timeout: 15000 })
  }
}
