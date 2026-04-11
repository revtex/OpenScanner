// Playwright spec — call upload triggers WS CAL event and scanner updates
import { test, expect } from '@playwright/test'

test.describe('Call upload flow', () => {
  test.todo('POST /api/call-upload with valid API key → 200')
  test.todo('WS CAL event received → scanner display updates')
  test.todo('call appears in history panel')
  test.todo('call appears in SEARCH results')
  test.todo('invalid API key → 401')
  test.todo('duplicate call within timeframe → rejected')
})
