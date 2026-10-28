import { test, expect } from '@playwright/test'
import { loginAs } from '../helpers/test-helpers'
import { AppShell } from '../pages/AppShell'

const PROGRAMS = ['pngquant', 'oxipng', 'optipng', 'jpegoptim', 'gifsicle']

test.describe('Settings', () => {
  test('lists every image tool and saves the global settings', async ({ page }) => {
    await loginAs(page, 'user')
    const shell = new AppShell(page)
    await shell.goto('/')
    await shell.navItem('settings').click()
    await expect(page.getByTestId('settings-page')).toBeVisible()

    for (const program of PROGRAMS) {
      await expect(page.getByTestId(`tool-status-${program}`)).toBeVisible()
      await expect(page.getByTestId(`tool-brew-${program}`)).toBeVisible()
    }
    await page.getByTestId('tools-refresh-button').click()
    await expect(page.getByTestId('tool-status-pngquant')).toBeVisible()

    const workers = page.getByTestId('settings-workers-input')
    await expect(workers).toHaveValue(/\d+/)
    const before = await workers.inputValue()
    await workers.fill('3')
    await page.getByTestId('settings-save-button').click()
    await page.reload()
    await expect(page.getByTestId('settings-workers-input')).toHaveValue('3')

    await page.getByTestId('settings-workers-input').fill(before)
    await page.getByTestId('settings-save-button').click()
    await page.reload()
    await expect(page.getByTestId('settings-workers-input')).toHaveValue(before)
  })
})
