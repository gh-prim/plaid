const v4 = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\/(\d{1,2})$/

function isV4(s: string): boolean {
  const m = v4.exec(s)
  if (!m) return false
  const [a, b, c, d, bits] = m.slice(1).map(Number) as [number, number, number, number, number]
  return [a, b, c, d].every((n) => n <= 255) && bits <= 32
}

function isV6(s: string): boolean {
  const parts = s.split('/')
  if (parts.length !== 2) return false
  const [addr = '', bits = ''] = parts
  if (!/^\d{1,3}$/.test(bits) || Number(bits) > 128) return false
  if (!/^[0-9a-f:]+$/i.test(addr) || (addr.match(/::/g) ?? []).length > 1) return false
  const groups = addr.split(':')
  const full = !addr.includes('::')
  if (full ? groups.length !== 8 : groups.length > 8) return false
  return groups.every((g) => g.length <= 4)
}

/** Whether `s` is an IPv4 or IPv6 network like 10.8.0.0/24 or fd00::/8. */
export function isCIDR(s: string): boolean {
  return isV4(s) || isV6(s)
}

/**
 * Splits "one network per line" (F007 R24) and returns the networks, and
 * the 1-based numbers of the invalid lines. Blank lines are ignored.
 */
export function parseNetworks(text: string): { networks: string[]; invalidLines: number[] } {
  const networks: string[] = []
  const invalidLines: number[] = []
  text.split('\n').forEach((line, i) => {
    const s = line.trim()
    if (!s) return
    if (isCIDR(s)) networks.push(s)
    else invalidLines.push(i + 1)
  })
  return { networks, invalidLines }
}
