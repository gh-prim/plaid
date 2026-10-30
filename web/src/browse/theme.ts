import { useCallback, useState } from 'react'

/** Appearance (F008-R4): dark by default; the player is always dark. */
export type Theme = 'system' | 'dark' | 'light'

const key = 'plaid.theme'

export function storedTheme(): Theme {
  try {
    const v = localStorage.getItem(key)
    if (v === 'system' || v === 'dark' || v === 'light') return v
  } catch {
    // Storage blocked: the default.
  }
  return 'dark'
}

/** Sets data-theme on <html>; "system" leaves it to prefers-color-scheme. */
export function applyTheme(t: Theme) {
  const root = document.documentElement
  if (t === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', t)
}

export function useTheme(): [Theme, (t: Theme) => void] {
  const [theme, setTheme] = useState(storedTheme)
  const set = useCallback((t: Theme) => {
    try {
      localStorage.setItem(key, t)
    } catch {
      // Not remembered, still applied.
    }
    applyTheme(t)
    setTheme(t)
  }, [])
  return [theme, set]
}
