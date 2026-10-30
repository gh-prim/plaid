import { describe, expect, it } from 'vitest'
import { isCIDR, parseNetworks } from './cidr'
import {
  durationFallback,
  formatBytes,
  formatClock,
  formatDuration,
  formatMbps,
  formatRelative,
  formatSpokenTime,
  nativeLanguageName,
} from './format'
import { mountCommand, setfaclCommand, shellQuote } from './shellQuote'

describe('shellQuote', () => {
  it('quotes a plain path', () => {
    expect(setfaclCommand('/srv/media/movies')).toBe(
      "sudo setfacl -R -m u:plaid:rX '/srv/media/movies'",
    )
  })
  it('keeps spaces, accents and double quotes as one argument', () => {
    expect(shellQuote('/media/pi/USB-4TB/Séries "VO"')).toBe(`'/media/pi/USB-4TB/Séries "VO"'`)
  })
  it('escapes a single quote so nothing runs', () => {
    expect(shellQuote(`/srv/Bob's films; rm -rf ~`)).toBe(`'/srv/Bob'\\''s films; rm -rf ~'`)
  })
  it('builds the mount check', () => {
    expect(mountCommand('/mnt/nas/Films')).toBe(
      `findmnt '/mnt/nas/Films' || sudo mount '/mnt/nas/Films'`,
    )
  })
})

describe('cidr', () => {
  it.each([
    '10.8.0.0/24',
    '192.168.1.0/32',
    '100.64.0.0/10',
    'fd00::/8',
    '2001:db8:0:0:0:0:0:0/32',
  ])('%s is valid', (s) => expect(isCIDR(s)).toBe(true))
  it.each(['10.8.0.0/33', '10.8.0/24', '256.1.1.1/8', '10.8.0.0', 'fd00::1::/8', 'hello'])(
    '%s is not',
    (s) => expect(isCIDR(s)).toBe(false),
  )
  it('reports invalid lines, ignoring blank ones', () => {
    expect(parseNetworks('10.8.0.0/24\n\n10.8.0.0/33\nfd00::/8')).toEqual({
      networks: ['10.8.0.0/24', 'fd00::/8'],
      invalidLines: [3],
    })
  })
})

describe('format', () => {
  it('formats sizes and bitrates per locale', () => {
    expect(formatBytes('en', 12_400_000_000)).toBe('12 GB')
    expect(formatBytes('fr', 5_300_000_000)).toMatch(/^5,3\s?Go$/)
    expect(formatMbps('en', 8_000_000)).toBe('8 Mb/s')
  })
  it('formats positions like a player', () => {
    expect(formatClock(4_368_000)).toBe('1:12:48')
    expect(formatClock(2_825_000)).toBe('47:05')
  })
  it('formats relative times', () => {
    const now = new Date('2026-10-29T12:00:00Z')
    expect(formatRelative('en', new Date('2026-10-29T11:57:00Z'), now)).toBe('3 minutes ago')
    expect(formatRelative('fr', new Date('2026-10-28T12:00:00Z'), now)).toBe('hier')
  })
  it('names each language in itself', () => {
    expect(nativeLanguageName('fr')).toBe('Français')
    expect(nativeLanguageName('en')).toBe('English')
  })
})

describe('formatDuration', () => {
  it('formats hours and minutes in the locale', () => {
    expect(formatDuration('en', 6_240_000)).toBe('1h 44m')
    expect(formatDuration('fr', 6_240_000)).toBe('1h 44min')
    expect(formatDuration('en', 600_000)).toBe('10m')
    expect(formatDuration('en', 7_200_000)).toBe('2h')
  })
  it('rounds to the minute, never shows 0 for something left', () => {
    expect(formatDuration('en', 20_000)).toBe('1m')
    expect(formatDuration('en', 0)).toBe('0m')
    expect(formatDuration('en', 89_000)).toBe('1m')
  })
  it('falls back without Intl.DurationFormat', () => {
    expect(durationFallback('en', { hours: 1, minutes: 44 })).toBe('1h 44m')
    expect(durationFallback('fr', { hours: 1, minutes: 44 })).toBe('1h 44min')
    expect(durationFallback('fr', { minutes: 10 })).toBe('10min')
  })
})

describe('formatSpokenTime', () => {
  it('says a position in words', () => {
    expect(formatSpokenTime('en', 3_733_000)).toMatch(/1 hour, 2 minutes,? 13 seconds/)
    expect(formatSpokenTime('en', 65_000)).toMatch(/1 minute,? 5 seconds/)
    expect(formatSpokenTime('fr', 0)).toMatch(/0\sseconde/)
  })
})
