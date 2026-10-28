import { expect, type Locator } from '@playwright/test'
import { BasePage } from './BasePage'

/** `/`: the folder cards. */
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

  enabledSwitch(card: Locator): Locator {
    return card.locator('[data-testid^="folder-enabled-switch-"]')
  }

  modeTag(card: Locator): Locator {
    return card.locator('[data-testid^="folder-mode-tag-"]')
  }

  editButton(card: Locator): Locator {
    return card.locator('[data-testid^="edit-folder-button-"]')
  }

  async gotoDashboard(): Promise<void> {
    await this.goto('/')
    await expect(this.root).toBeVisible()
  }
}
