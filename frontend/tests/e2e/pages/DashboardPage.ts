import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/** `/`: the overview tiles, the savings chart and the folder cards. */
export class DashboardPage extends BasePage {
  get root(): Locator {
    return this.page.getByTestId('dashboard-page')
  }

  get addFolderButton(): Locator {
    return this.page.getByTestId('add-folder-button')
  }

  folderCard(name: string): Locator {
    return this.page.locator('[data-testid^="folder-card-"]').filter({ hasText: name })
  }

  /** The card's name, a link to the folder's page. */
  folderLink(card: Locator): Locator {
    return card.locator('[data-testid^="folder-link-"]')
  }

  /** An overview tile; its figure across every folder is in `data-value`. */
  overview(tile: 'saved' | 'optimized' | 'pending' | 'failed' | 'optimizing' | 'scans'): Locator {
    return this.page.getByTestId(`overview-${tile}`)
  }

  /** The chart's day columns, oldest first. */
  get savingsDays(): Locator {
    return this.page.getByTestId('savings-chart').locator('[data-testid^="savings-day-"]')
  }

  get savingsTooltip(): Locator {
    return this.page.getByTestId('savings-tooltip')
  }

  enabledSwitch(card: Locator): Locator {
    return card.locator('[data-testid^="folder-enabled-switch-"]')
  }

  modeTag(card: Locator): Locator {
    return card.locator('[data-testid^="folder-mode-tag-"]')
  }

  editButton(card: Locator): Locator {
    return card.locator('[data-testid^="edit-folder-button-"]')
  }

  imageCount(card: Locator): Locator {
    return card.locator('[data-testid^="folder-image-count-"]')
  }

  scanStatus(card: Locator): Locator {
    return card.locator('[data-testid^="folder-scan-status-"]')
  }

  /** The optimized count (`data-count`) and the bytes saved (`data-bytes`). */
  optimized(card: Locator): Locator {
    return card.locator('[data-testid^="folder-optimized-"]')
  }

  /** Opens the card's Scan now menu and picks a quick or full scan. */
  async scan(card: Locator, kind: 'quick' | 'full'): Promise<void> {
    await card.locator('[data-testid^="scan-folder-button-"]').click()
    await this.page.locator(`[data-testid^="${kind}-scan-item-"]`).click()
  }

  async gotoDashboard(): Promise<void> {
    await this.goto('/')
    await expect(this.root).toBeVisible()
  }
}
