import { useTranslation } from 'react-i18next'
import type { components } from '../api/schema'
import { resolutionName } from '../browse/itemFacts'
import { formatMbps } from '../lib/format'
import type { CapabilityProfile } from './capabilities'
import { audioLabel, languageLabel, methodLabel, qualityLabel, reasonLabel } from './labels'
import { Panel } from './TrackMenus'

type Session = components['schemas']['PlaybackSession']
type MediaFile = components['schemas']['MediaFile']

/**
 * "Playback info" (F008-R41): how the video reaches you and why, in
 * words, so "Converting" makes sense without the admin.
 */
export function InfoPanel({
  session,
  file,
  subtitle,
  profile,
  onClose,
}: {
  session: Session
  file?: MediaFile
  /** The text subtitle shown, if any. */
  subtitle: number | null
  profile: CapabilityProfile
  onClose: () => void
}) {
  const { t, i18n } = useTranslation()
  const lng = i18n.language
  const rows: [string, string][] = [[t('player.info.method'), methodLabel(t, session.method)]]
  const why = [...session.reasons, ...session.warnings].map((r) => reasonLabel(t, r))
  if (why.length) rows.push([t('player.info.why'), why.join(' ')])

  const source = [
    resolutionName(file?.width, file?.height),
    file?.bitrate && formatMbps(lng, file.bitrate),
  ]
    .filter(Boolean)
    .join(' · ')
  rows.push([
    t('player.info.quality'),
    session.quality === 'original' && source
      ? t('player.quality.current', { quality: qualityLabel(t, 'original'), bitrate: source })
      : qualityLabel(t, session.quality),
  ])
  if (session.method === 'transcode')
    rows.push([
      t('player.info.conversion'),
      session.hardwareAcceleration ? t('player.info.hardware') : t('player.info.software'),
    ])

  const audio = session.audioTracks.find((a) => a.selected)
  if (audio) {
    const track = audioLabel(t, lng, audio)
    rows.push([
      t('player.audio.label'),
      audio.delivered
        ? t('player.info.audioConverted', { track, codec: audio.delivered.toUpperCase() })
        : t('player.info.copied', { track }),
    ])
  }
  const burned = session.subtitles.find((s) => s.selected && s.delivery === 'burn_in')
  const text = session.subtitles.find((s) => s.index === subtitle)
  rows.push([
    t('player.subtitles.label'),
    burned
      ? reasonLabel(t, 'subtitle_burn_in')
      : text
        ? t('player.info.textSubs', { language: languageLabel(t, lng, text.language) })
        : t('player.subtitles.off'),
  ])
  const codecs = (profile.video ?? []).map((v) => v.codec.toUpperCase().replace('H264', 'H.264'))
  if (codecs.length)
    rows.push([
      t('player.info.browser'),
      new Intl.ListFormat(lng, { type: 'conjunction' }).format(codecs),
    ])

  return (
    <Panel title={t('player.info.label')} onClose={onClose} wide>
      <dl className="pv-info">
        {rows.map(([k, v]) => (
          <div key={k} className="pv-info__row">
            <dt>{k}</dt>
            <dd>{v}</dd>
          </div>
        ))}
      </dl>
    </Panel>
  )
}
