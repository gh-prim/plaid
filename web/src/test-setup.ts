import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, vi } from 'vitest'

// Tests never reach the network: a request still in flight when a test
// ends (a poll, a page loading) would otherwise go to the real fetch once
// fakeApi's stub is gone and fail as an unhandled ECONNREFUSED.
globalThis.fetch = () => new Promise<Response>(() => {})

afterEach(() => {
  cleanup()
  localStorage.clear()
  sessionStorage.clear()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})
