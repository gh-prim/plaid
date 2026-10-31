import { fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fakeVideo } from '../test/fakeVideo'
import { renderUi } from '../test/renderUi'
import { HeroPreview } from './HeroPreview'

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => ({ state: { status: 'ready', me: { activeProfile: { id: 'p1' } } } }),
}))

// Waits out the 2 s of backdrop before the scenes show.
const shown = { timeout: 4000 }

describe('hero preview', () => {
  let video: ReturnType<typeof fakeVideo>
  beforeEach(() => {
    localStorage.clear()
    video = fakeVideo()
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        constructor(private cb: (e: { isIntersecting: boolean }[]) => void) {}
        observe() {
          this.cb([{ isIntersecting: true }])
        }
        disconnect() {}
      },
    )
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(new Response(null, { status: 200 }))),
    )
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  const el = () => document.querySelector('video') as HTMLVideoElement

  it('plays with sound, and the sound button mutes it for the profile', async () => {
    await renderUi(<HeroPreview itemId="m1" />)
    const mute = await screen.findByRole('button', { name: 'Mute the preview' }, shown)
    expect(video.state(el()).muted).toBe(false)
    fireEvent.click(mute)
    await waitFor(() => expect(video.state(el()).muted).toBe(true))
    expect(screen.getByRole('button', { name: 'Turn the preview’s sound on' })).toBeInTheDocument()
    expect(localStorage.getItem('plaid.previewSound.p1')).toBe('0')
  })

  it('plays muted when the browser refuses sound, until the sound button', async () => {
    // Refuses sound until a click; plays muted.
    let clicked = false
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(function (
      this: HTMLMediaElement,
    ) {
      if (!this.muted && !clicked) return Promise.reject(new DOMException('no', 'NotAllowedError'))
      video.state(this).paused = false
      this.dispatchEvent(new Event('playing'))
      return Promise.resolve()
    })
    await renderUi(<HeroPreview itemId="m1" />)
    const unmute = await screen.findByRole('button', { name: 'Turn the preview’s sound on' }, shown)
    expect(video.state(el()).muted).toBe(true)
    // The choice stays "sound on": only this page plays muted.
    expect(localStorage.getItem('plaid.previewSound.p1')).toBeNull()
    clicked = true
    fireEvent.click(unmute)
    expect(video.state(el()).muted).toBe(false)
    expect(await screen.findByRole('button', { name: 'Mute the preview' })).toBeInTheDocument()
  })
})
