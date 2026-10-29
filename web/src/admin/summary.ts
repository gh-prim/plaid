import type { TFunction } from 'i18next'
import type { components } from '../api/schema'
import type { Tone } from '../ui'

type ActivePlayback = components['schemas']['ActivePlayback']
type Library = components['schemas']['Library']
type Capabilities = components['schemas']['Capabilities']
type Problem = components['schemas']['Problem']

export type AdminData = {
  streams: ActivePlayback[]
  libraries: Library[]
  capabilities?: Capabilities
  problems: Problem[]
}

export type Need = { key: string; title: string; body?: string; to: string; tone: Tone }

/** Under 1.0× a transcode can't keep up: it will buffer. */
export const slow = (s: ActivePlayback) =>
  s.session.method === 'transcode' && s.speed != null && s.speed < 1

/**
 * What needs the admin right now (F007 R18): unavailable or unreadable
 * folders, a stream that can't keep up, a hardware fallback, FFmpeg
 * missing, and open problems not already covered by those.
 */
export function needs(t: TFunction, d: AdminData): Need[] {
  const out: Need[] = []
  for (const l of d.libraries) {
    for (const f of l.folders) {
      if (f.status === 'unavailable') {
        out.push({
          key: `u-${f.id}`,
          title: t('folder.unavailable.title', { path: f.path }),
          body: l.name,
          to: `/admin/libraries/${l.id}`,
          tone: 'error',
        })
      } else if (f.status === 'unreadable') {
        out.push({
          key: `r-${f.id}`,
          title: t('errors.folder_unreadable'),
          body: f.path,
          to: `/admin/libraries/${l.id}`,
          tone: 'error',
        })
      }
    }
  }
  for (const s of d.streams.filter(slow)) {
    out.push({
      key: `s-${s.session.id}`,
      title: t('admin.needs.slow', { title: s.itemTitle }),
      body: s.profileName,
      to: '/admin/playing',
      tone: 'warning',
    })
  }
  const hw = d.capabilities?.hwAccel
  if (hw?.disabledReason === 'repeated_failures') {
    out.push({
      key: 'hw',
      title: t('admin.needs.hwFallback'),
      to: '/admin/transcoding',
      tone: 'warning',
    })
  }
  if (d.capabilities && !d.capabilities.ffmpeg.found) {
    out.push({
      key: 'ffmpeg',
      title: t('transcoding.ffmpeg.missing'),
      to: '/admin/transcoding',
      tone: 'warning',
    })
  }
  const covered = new Set(['folder_unavailable', 'folder_unreadable', 'hw_fallback'])
  const open = d.problems.filter((p) => !p.resolvedAt && !covered.has(p.code))
  if (open.length) {
    out.push({
      key: 'problems',
      title: t('admin.needs.problems', { count: open.length }),
      to: '/admin/problems',
      tone: 'warning',
    })
  }
  return out
}

/** The one sentence under the server's name (F007 R18). */
export function statusLine(t: TFunction, d: AdminData, needCount: number): string {
  if (needCount && !d.streams.length) return t('admin.home.status.problems', { count: needCount })
  if (!d.streams.length) return t('admin.home.status.quiet')
  if (d.streams.some(slow)) return t('admin.home.status.slow', { count: d.streams.length })
  return t('admin.home.status.playing', { count: d.streams.length })
}
