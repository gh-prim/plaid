import { describe, expect, it } from 'vitest'
import type { components } from '../api/schema'
import { adminStep, doneKey, laterKey, setFlag } from './step'

type Library = components['schemas']['Library']
const lib = (lastScanAt?: string) => ({ id: 'l', name: 'Movies', lastScanAt }) as Library

describe('adminStep', () => {
  it('sends an admin with no library to the libraries step', () => {
    expect(adminStep([], true)).toBe('libraries')
  })
  it('respects "later" on this browser', () => {
    setFlag(laterKey)
    expect(adminStep([], true)).toBe('done')
  })
  it('follows a library that never finished a scan', () => {
    expect(adminStep([lib('2026-10-29T10:00:00Z'), lib()], true)).toBe('scan')
  })
  it('is done once every library has been scanned', () => {
    expect(adminStep([lib('2026-10-29T10:00:00Z')], true)).toBe('done')
  })
  it('is done once the admin left the end screen', () => {
    setFlag(doneKey)
    expect(adminStep([lib()], true)).toBe('done')
  })
  it('never shows members the wizard', () => {
    expect(adminStep([], false)).toBe('done')
  })
})
