import { expect, test } from '@playwright/test'

test('public legal pages are reachable', async ({ page }) => {
  await page.goto('/privacy')
  await expect(page.getByRole('heading', { name: 'Privacy Policy' })).toBeVisible()
  await expect(page.getByText('Google Gemini')).toBeVisible()

  await page.goto('/terms')
  await expect(page.getByRole('heading', { name: 'Terms of Service' })).toBeVisible()
  await expect(page.getByText('usage limits')).toBeVisible()
})

test('unauthenticated users are redirected away from protected reader surfaces', async ({ page }) => {
  await page.goto('/documents')
  await expect(page).toHaveURL(/\/login$/)

  await page.goto('/history')
  await expect(page).toHaveURL(/\/login$/)
})
