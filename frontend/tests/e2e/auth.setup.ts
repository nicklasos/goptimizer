import { test as setup, expect } from '@playwright/test'
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'
import dbHelper from './helpers/db-helper'
import { LoginPage } from './pages/LoginPage'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const authDir = path.join(__dirname, '.auth')

fs.mkdirSync(authDir, { recursive: true })

setup('authenticate as user', async ({ page }) => {
  const user = await dbHelper.createUser({ name: 'E2E Setup User' })

  const loginPage = new LoginPage(page)
  await loginPage.login(user.email, user.password)
  await expect(page.getByTestId('main-content')).toBeVisible()

  await page.context().storageState({ path: path.join(authDir, 'user.json') })
})
