import type { TFunction } from 'i18next'
import type { components } from '../api/schema'
import { languageName } from '../lib/format'

type AudioTrack = components['schemas']['PlaybackAudioTrack']
type Subtitle = components['schemas']['PlaybackSubtitle']
type Quality = components['schemas']['Quality']

/** A track's language as a menu label: "French", "Français". */
export function languageLabel(t: TFunction, locale: string, code: string): string {
  if (!code || code === 'und') return t('player.track.unknown')
  const name = languageName(locale, code)
  return name.charAt(0).toLocaleUpperCase(locale) + name.slice(1)
}

export function channelsLabel(t: TFunction, channels: number): string {
  if (channels <= 1) return t('player.channels.mono')
  if (channels === 2) return t('player.channels.stereo')
  const layout = channels === 6 ? '5.1' : channels === 8 ? '7.1' : String(channels)
  return t('player.channels.surround', { layout })
}

export function audioLabel(t: TFunction, locale: string, a: AudioTrack): string {
  return t('player.audio.track', {
    language: languageLabel(t, locale, a.language),
    channels: channelsLabel(t, a.channels),
  })
}

/** "English", "French (forced)", "English SDH". */
export function subtitleLabel(t: TFunction, locale: string, s: Subtitle, sdh: boolean): string {
  const language = languageLabel(t, locale, s.language)
  if (s.forced) return t('player.subtitles.forced', { language })
  if (sdh) return t('player.subtitles.sdhName', { language })
  return language
}

export function qualityLabel(t: TFunction, q: Quality): string {
  return q === 'original' ? t('player.quality.original') : q
}

/** The words for a playback method (F006 keys). */
export function methodLabel(t: TFunction, m: components['schemas']['PlaybackMethod']): string {
  switch (m) {
    case 'direct_play':
      return t('playback.method.direct_play')
    case 'direct_stream':
      return t('playback.method.direct_stream')
    case 'transcode':
      return t('playback.method.transcode')
  }
}

/** A reason or warning key as a sentence (shared with F007). */
export function reasonLabel(t: TFunction, key: string): string {
  switch (key) {
    case 'audio_channels_too_many':
      return t('reason.audio_channels_too_many')
    case 'audio_codec_not_supported':
      return t('reason.audio_codec_not_supported')
    case 'audio_track_switch_unsupported':
      return t('reason.audio_track_switch_unsupported')
    case 'bit_depth_not_supported':
      return t('reason.bit_depth_not_supported')
    case 'bitrate_too_high':
      return t('reason.bitrate_too_high.connection')
    case 'colors_may_be_wrong':
      return t('reason.colors_may_be_wrong')
    case 'container_not_supported':
      return t('reason.container_not_supported')
    case 'forced_by_client':
      return t('reason.forced_by_client')
    case 'hdr_not_supported':
      return t('reason.hdr_not_supported')
    case 'keyframe_index_missing':
      return t('reason.keyframe_index_missing')
    case 'resolution_too_high':
      return t('reason.resolution_too_high')
    case 'subtitle_burn_in':
      return t('reason.subtitle_burn_in')
    case 'video_codec_not_supported':
      return t('reason.video_codec_not_supported')
    case 'video_profile_not_supported':
      return t('reason.video_profile_not_supported')
  }
  return t('reason.other')
}
