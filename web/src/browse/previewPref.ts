import { useSyncExternalStore } from 'react'

/**
 * "Play previews automatically" (V2 hero), per profile in this browser,
 * on by default. The hero follows the switch at once.
 */
const key = (profileId: string) => `plaid.previews.${profileId}`
const listeners = new Set<() => void>()

export function previewsOn(profileId: string): boolean {
  try {
    return localStorage.getItem(key(profileId)) !== '0'
  } catch {
    return true
  }
}

export function setPreviewsOn(profileId: string, on: boolean) {
  try {
    localStorage.setItem(key(profileId), on ? '1' : '0')
  } catch {
    // Not remembered: this tab still follows.
  }
  listeners.forEach((l) => l())
}

function subscribe(l: () => void) {
  listeners.add(l)
  return () => {
    listeners.delete(l)
  }
}

export function usePreviewsOn(profileId: string): boolean {
  return useSyncExternalStore(subscribe, () => previewsOn(profileId))
}

/**
 * Whether previews play with their sound, per profile in this browser,
 * on by default; the hero's sound button flips it.
 */
const soundKey = (profileId: string) => `plaid.previewSound.${profileId}`

export function previewSoundOn(profileId: string): boolean {
  try {
    return localStorage.getItem(soundKey(profileId)) !== '0'
  } catch {
    return true
  }
}

export function setPreviewSoundOn(profileId: string, on: boolean) {
  try {
    localStorage.setItem(soundKey(profileId), on ? '1' : '0')
  } catch {
    // Not remembered: this tab still follows.
  }
  listeners.forEach((l) => l())
}

export function usePreviewSoundOn(profileId: string): boolean {
  return useSyncExternalStore(subscribe, () => previewSoundOn(profileId))
}
