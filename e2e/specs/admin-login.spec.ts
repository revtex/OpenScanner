// Playwright spec — admin login flow
import { test, expect } from '@playwright/test'

test.describe('Admin login', () => {
  test.fixme('correct password → dashboard loads', async () => {})
  test.fixme('wrong password → error shown', async () => {})
  test.fixme('3 wrong passwords → rate limited (429)', async () => {})
  test.fixme('passwordNeedChange=true → redirected to change-password', async () => {})
})
