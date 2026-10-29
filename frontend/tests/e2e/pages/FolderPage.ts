import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

export type ImageTab = 'all' | 'pending' | 'optimizing' | 'optimized' | 'kept' | 'skipped' | 'failed' | 'gone'
export type StatTile = 'saved' | 'optimized' | 'pending' | 'failed' | 'optimizing' | 'scans'

/** `/folders/:id`: one folder's settings, figures, images and scans. */
export class FolderPage extends BasePage {
  get root(): Locator {
    return this.page.getByTestId('folder-page')
  }

  get title(): Locator {
    return this.page.getByTestId('folder-page-title')
  }

  get notFound(): Locator {
    return this.page.getByTestId('page-not-found')
  }

  /** A tile's figure is in `data-value`. */
  stat(tile: StatTile): Locator {
    return this.page.getByTestId(`folder-stats-${tile}`)
  }

  /** One day's column of the chart: `data-bytes` and `data-count`. */
  savingsDay(day: string): Locator {
    return this.page.getByTestId('folder-savings-chart').getByTestId(`savings-day-${day}`)
  }

  /** A status tab; its image count is in `data-count`. */
  tab(tab: ImageTab): Locator {
    return this.page.getByTestId(`images-tab-${tab}`)
  }

  /** The label of the status tab that is selected. */
  selectedTab(): Locator {
    return this.page.getByRole('tab', { selected: true }).locator('[data-testid^="images-tab-"]')
  }

  get searchInput(): Locator {
    return this.page.getByTestId('images-search-input')
  }

  get imagesTable(): Locator {
    return this.page.getByTestId('images-table')
  }

  get imageRows(): Locator {
    return this.imagesTable.locator('tr[data-row-key]')
  }

  imageRow(id: number): Locator {
    return this.imagesTable.locator(`tr[data-row-key="${id}"]`)
  }

  /** The reason cell: `data-reason-code` and `data-lossless-code`. */
  reason(id: number): Locator {
    return this.page.getByTestId(`image-reason-${id}`)
  }

  imageStatus(id: number): Locator {
    return this.page.getByTestId(`image-status-${id}`)
  }

  retryButton(id: number): Locator {
    return this.page.getByTestId(`retry-image-button-${id}`)
  }

  get retryAllButton(): Locator {
    return this.page.getByTestId('retry-failed-images-button')
  }

  get loadMoreButton(): Locator {
    return this.page.getByTestId('images-load-more-button')
  }

  get scanRows(): Locator {
    return this.page.getByTestId('scans-table').locator('tr[data-row-key]')
  }

  scanStatus(id: number): Locator {
    return this.page.getByTestId(`scan-status-${id}`)
  }

  /** The ids of the image rows on screen, in order. */
  async imageIds(): Promise<number[]> {
    return (await this.imageRows.evaluateAll((rows) => rows.map((row) => row.getAttribute('data-row-key')))).map(Number)
  }

  async open(id: number, query = ''): Promise<void> {
    await this.goto(`/folders/${id}${query}`)
    await expect(this.root).toBeVisible()
  }
}
