import type { Locator } from '@playwright/test'
import { BasePage } from './BasePage'

export class StorageBrowserPage extends BasePage {
  get table(): Locator {
    return this.page.getByTestId('storage-objects-table')
  }

  get rows(): Locator {
    return this.table.locator('tr[data-row-key]')
  }

  get error(): Locator {
    return this.page.getByTestId('storage-browser-error')
  }

  folder(name: string): Locator {
    return this.page.getByTestId(`storage-folder-${name}`)
  }

  file(name: string): Locator {
    return this.page.getByTestId(`storage-file-${name}`)
  }

  crumb(name: string): Locator {
    return this.page.getByTestId(`storage-crumb-${name}`)
  }
}
