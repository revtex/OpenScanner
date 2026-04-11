// Playwright spec — admin login flow
import { test, expect } from '@playwright/test'

test.describe('Admin login', () => {
  test.todo('correct password → dashboard loads')
  test.todo('wrong password → error shown')
  test.todo('3 wrong passwords → rate limited (429)')
  test.todo('passwordNeedChange=true → redirected to change-password')
})
