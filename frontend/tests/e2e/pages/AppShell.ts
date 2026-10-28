import { expect, type Locator } from '@playwright/test'
import { AUTH_STORAGE_KEY } from '../helpers/test-helpers'
import { BasePage } from './BasePage'

export class AppShell extends BasePage {
  get userMenuTrigger(): Locator {
    return this.page.getByTestId('user-menu-button')
  }

  get mainMenu(): Locator {
    return this.page.getByTestId('main-menu')
  }

  get mobileMenuToggle(): Locator {
    return this.page.getByTestId('menu-toggle-button')
  }

  navItem(key: string): Locator {
    return this.page.getByTestId(`nav-${key}`)
  }

  userMenuItem(key: string): Locator {
    return this.page.getByTestId(`user-menu-${key}`)
  }

  async openUserMenu(): Promise<void> {
    await this.userMenuTrigger.click()
    await expect(this.userMenuItem('profile')).toBeVisible()
  }

  async openProfile(): Promise<void> {
    await this.openUserMenu()
    await this.userMenuItem('profile').click()
    await expect(this.page.getByTestId('profile-page')).toBeVisible()
  }

  async logoutViaUI(): Promise<void> {
    await this.openUserMenu()
    await this.userMenuItem('logout').click()
    await expect(this.loginEmailInput).toBeVisible({ timeout: 10000 })
  }

  async clearAuthStorage(): Promise<void> {
    await this.page.evaluate((key) => localStorage.removeItem(key), AUTH_STORAGE_KEY)
  }
}
