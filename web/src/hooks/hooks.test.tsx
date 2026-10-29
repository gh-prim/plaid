import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { apiError, fakeApi } from '../test/fakeApi'
import { FakeEventSource, stubEventSource } from '../test/fakeEventSource'
import { useConnection } from './useConnection'
import { usePolling } from './usePolling'
import { keptMatches, useScanEvents } from './useScanEvents'

const progress = (state: string, found: number, analyzed: number) => ({
  state,
  queued: false,
  full: false,
  counters: {
    found,
    new: found,
    changed: 0,
    moved: 0,
    missing: 0,
    analyzed,
    failed: 0,
    unrecognized: 0,
  },
})

describe('useScanEvents', () => {
  it('follows progress, metadata counters and matched items', async () => {
    stubEventSource()
    const { result, unmount } = renderHook(() => useScanEvents('lib 1'))
    const es = FakeEventSource.last!
    expect(es.url).toBe('/api/v1/libraries/lib%201/scan/events')

    act(() => {
      es.emit('progress', { type: 'progress', progress: progress('analyzing', 1263, 806) })
      es.emit('metadata', {
        type: 'metadata',
        metadata: { pending: 840, matched: 412, needsReview: 18, unmatched: 3 },
      })
      es.emit('matched', {
        type: 'matched',
        item: { id: 'a', kind: 'movie', title: 'Alien', year: 1979 },
      })
      es.emit('matched', {
        type: 'matched',
        item: { id: 'b', kind: 'movie', title: 'Dune: Part Two', year: 2024 },
      })
      es.emit('matched', {
        type: 'matched',
        item: { id: 'a', kind: 'movie', title: 'Alien', year: 1979 },
      })
    })
    expect(result.current.progress?.counters.analyzed).toBe(806)
    expect(result.current.metadata?.matched).toBe(412)
    expect(result.current.matched.map((m) => m.id)).toEqual(['b', 'a'])

    unmount()
    expect(es.closed).toBe(true)
  })

  it(`keeps the ${keptMatches} latest matches`, () => {
    stubEventSource()
    const { result } = renderHook(() => useScanEvents('lib'))
    act(() => {
      for (let i = 0; i < keptMatches + 10; i++) {
        FakeEventSource.last!.emit('matched', {
          type: 'matched',
          item: { id: String(i), kind: 'movie', title: `M${i}` },
        })
      }
    })
    expect(result.current.matched).toHaveLength(keptMatches)
    expect(result.current.matched[0]?.id).toBe(String(keptMatches + 9))
  })

  it('reports a failed scan', () => {
    stubEventSource()
    const { result } = renderHook(() => useScanEvents('lib'))
    act(() =>
      FakeEventSource.last!.emit('failed', {
        type: 'failed',
        error: 'disk gone',
        progress: progress('idle', 0, 0),
      }),
    )
    expect(result.current.failed).toBe('disk gone')
  })

  it('opens nothing without a library', () => {
    stubEventSource()
    renderHook(() => useScanEvents(undefined))
    expect(FakeEventSource.all).toHaveLength(0)
  })
})

describe('usePolling', () => {
  it('polls while visible and stops when the tab is hidden', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    let hidden = false
    vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
    const load = vi.fn().mockResolvedValue(1)
    renderHook(() => usePolling(load, 5000))
    await waitFor(() => expect(load).toHaveBeenCalledTimes(1))
    await act(() => vi.advanceTimersByTimeAsync(5000))
    expect(load).toHaveBeenCalledTimes(2)

    hidden = true
    act(() => document.dispatchEvent(new Event('visibilitychange')))
    await act(() => vi.advanceTimersByTimeAsync(20000))
    expect(load).toHaveBeenCalledTimes(2)

    hidden = false
    act(() => document.dispatchEvent(new Event('visibilitychange')))
    await waitFor(() => expect(load).toHaveBeenCalledTimes(3))
    vi.useRealTimers()
  })
})

describe('useConnection', () => {
  it('retries on the ladder until /health answers, then says it is back', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    let up = false
    const calls = fakeApi({
      'GET /health': () =>
        up ? { status: 200, body: { status: 'ok' } } : apiError(503, 'unavailable'),
    })
    const { result } = renderHook(() => useConnection(1000))
    act(() => result.current.lost())
    await waitFor(() =>
      expect(result.current.connection).toEqual({ status: 'offline', retryInSeconds: 2 }),
    )

    await act(() => vi.advanceTimersByTimeAsync(2000))
    await waitFor(() =>
      expect(result.current.connection).toMatchObject({ status: 'offline', retryInSeconds: 4 }),
    )

    up = true
    await act(() => vi.advanceTimersByTimeAsync(4000))
    await waitFor(() => expect(result.current.connection).toEqual({ status: 'back' }))
    await act(() => vi.advanceTimersByTimeAsync(1000))
    expect(result.current.connection).toEqual({ status: 'online' })
    expect(calls.filter((c) => c.path === '/health').length).toBe(3)
    vi.useRealTimers()
  })
})
