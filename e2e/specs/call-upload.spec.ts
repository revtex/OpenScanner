// Playwright spec — call upload triggers WS CAL event and scanner updates
import { test, expect } from '@playwright/test'

test.describe('Call upload flow', () => {
  test.fixme('POST /api/call-upload with valid API key → 200', async () => {})
  test.fixme('WS CAL event received → scanner display updates', async () => {})
  test.fixme('call appears in history panel', async () => {})
  test.fixme('call appears in SEARCH results', async () => {})
  test.fixme('invalid API key → 401', async () => {})
  test.fixme('duplicate call within timeframe → rejected', async () => {})
})
