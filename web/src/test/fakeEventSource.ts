import { vi } from 'vitest'

/** A stand-in for EventSource: tests push server events with `emit`. */
export class FakeEventSource {
  static last: FakeEventSource | undefined
  static all: FakeEventSource[] = []
  url: string
  closed = false
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  private listeners = new Map<string, ((e: MessageEvent<string>) => void)[]>()

  constructor(url: string) {
    this.url = url
    FakeEventSource.last = this
    FakeEventSource.all.push(this)
  }

  addEventListener(type: string, fn: (e: MessageEvent<string>) => void) {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn])
  }

  close() {
    this.closed = true
  }

  /** Sends one event as the server would: `event: <type>` with JSON data. */
  emit(type: string, data: unknown) {
    const e = new MessageEvent(type, { data: JSON.stringify(data) })
    for (const fn of this.listeners.get(type) ?? []) fn(e)
  }
}

export function stubEventSource() {
  FakeEventSource.last = undefined
  FakeEventSource.all = []
  vi.stubGlobal('EventSource', FakeEventSource)
}
