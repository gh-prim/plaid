import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { usePolling } from '../hooks/usePolling'
import { formatClock } from '../lib/format'
import { Button, Dialog, StreamCard } from '../ui'
import { streamPoster, useAdmin } from './AdminLayout'
import { Loading, Section } from './v2'
import { methodText, reasonText } from './reasons'
import { slow } from './summary'

type ActivePlayback = components['schemas']['ActivePlayback']

/** Playing now refreshes every 5 s while visible (F007 R19). */
export const playingEvery = 5000

async function load() {
  const [p, s] = await Promise.all([api.GET('/playback'), api.GET('/settings/playback')])
  return {
    streams: p.data?.sessions ?? [],
    remoteMaxBitrate: s.data?.remoteMaxBitrate ?? 0,
    maxTranscodes: s.data?.maxTranscodes,
  }
}

/** Slowest first: a stream that can't keep up, then transcodes, then the rest. */
export function bySeverity(a: ActivePlayback, b: ActivePlayback): number {
  const rank = (s: ActivePlayback) =>
    slow(s)
      ? 0
      : s.session.method === 'transcode'
        ? 1
        : s.session.method === 'direct_stream'
          ? 2
          : 3
  return rank(a) - rank(b) || (a.speed ?? 9) - (b.speed ?? 9)
}

/** Who's watching what, and why each stream is converted (boards AP03, AP04, AD03). */
export function Playing() {
  const { t } = useTranslation()
  const { refresh: refreshSummary, toast } = useAdmin()
  const { data, refresh } = usePolling(load, playingEvery)
  const [stopping, setStopping] = useState<ActivePlayback>()
  const streams = [...(data?.streams ?? [])].sort(bySeverity)
  const converting = streams.filter((s) => s.session.method === 'transcode').length
  const limitMbps = data?.remoteMaxBitrate ? data.remoteMaxBitrate / 1e6 : undefined

  async function stop(s: ActivePlayback) {
    setStopping(undefined)
    await api.DELETE('/playback/{sessionId}', { params: { path: { sessionId: s.session.id } } })
    toast(t('playing.stopped', { title: s.itemTitle, user: s.profileName }))
    await Promise.all([refresh(), refreshSummary()])
  }

  return (
    <Section
      title={t('playing.title')}
      glyph="playing"
      paneTitle={data ? t('admin.summary.playing', { count: streams.length }) : undefined}
      paneSub={
        data && data.maxTranscodes != null
          ? `${t('admin.summary.playing', { count: streams.length })} · ${t('playing.capacity', { count: converting, max: data.maxTranscodes })}`
          : undefined
      }
      status={
        streams.some(slow)
          ? { text: t('admin.playing.slow'), tone: 'warning' }
          : streams.length
            ? { text: t('admin.playing.keepUp'), tone: 'ok' }
            : undefined
      }
    >
      {!data && <Loading />}
      {data && streams.length === 0 && (
        <div className="adm__empty">
          <p className="title">{t('playing.empty.title')}</p>
          <p className="adm__text">{t('playing.empty.label')}</p>
        </div>
      )}
      <ul className="adm__grid">
        {streams.map((s) => {
          const advice =
            slow(s) && !s.session.hardwareAcceleration
              ? t('playing.slow.advice.software', { user: s.profileName })
              : undefined
          return (
            <li key={s.session.id}>
              <StreamCard
                title={s.itemTitle}
                poster={streamPoster(s, 154)}
                who={t('playing.row', {
                  user: s.username,
                  profile: s.profileName,
                  device: s.deviceName,
                })}
                position={`${t('playing.position', { position: formatClock(s.positionMs), duration: formatClock(s.session.durationMs) })} · ${t(s.remote ? 'playing.where.remote' : 'playing.where.local')}`}
                progress={
                  s.session.durationMs ? (100 * s.positionMs) / s.session.durationMs : undefined
                }
                method={s.session.method}
                methodLabel={`${methodText(t, s.session.method)} · ${s.session.method === 'transcode' ? (s.session.hardwareAcceleration ? t('playing.hw.hardware', { accelerator: s.session.hardwareAcceleration.toUpperCase() }) : t('playing.hw.software')) : ''}`.replace(
                  / · $/,
                  '',
                )}
                reasons={s.session.reasons.map((r) =>
                  reasonText(t, r, { remote: s.remote, limitMbps }),
                )}
                quality={s.session.quality === 'original' ? undefined : s.session.quality}
                speed={s.speed}
                advice={advice}
                onStop={() => setStopping(s)}
              />
            </li>
          )
        })}
      </ul>
      {stopping && (
        <Dialog
          title={t('playing.stop.confirm.label', {
            title: stopping.itemTitle,
            user: stopping.profileName,
          })}
          actions={
            <>
              <Button variant="ghost" onClick={() => setStopping(undefined)}>
                {t('playing.stop.confirm.cancel')}
              </Button>
              <Button onClick={() => void stop(stopping)}>
                {t('playing.stop.confirm.action')}
              </Button>
            </>
          }
        />
      )}
    </Section>
  )
}
