import { useEffect, useState } from 'react'
import { PartySocket } from './socket'

// Notifications over the socket (F010-R41): one shared connection while the
// app is open; screens that list parties reload when something changes.

let shared: PartySocket | null = null
let users = 0
const listeners = new Set<(kind: string) => void>()

function open() {
  if (shared) return
  shared = new PartySocket()
  shared.on<{ kind: string }>('notify', (d) => {
    for (const l of listeners) l(d.kind)
  })
  shared.connect()
}

/** A counter that moves on every notification, for effects to reload. */
export function useNotify(): number {
  const [tick, setTick] = useState(0)
  useEffect(() => {
    users++
    open()
    const l = () => setTick((n) => n + 1)
    listeners.add(l)
    return () => {
      listeners.delete(l)
      if (--users === 0) {
        shared?.close()
        shared = null
      }
    }
  }, [])
  return tick
}
