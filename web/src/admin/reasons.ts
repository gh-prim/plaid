import type { TFunction } from 'i18next'

/** Every F006 decision reason the server can send (F006 R5, R23, F007 R19). */
export const reasonCodes = [
  'container_not_supported',
  'video_codec_not_supported',
  'video_profile_not_supported',
  'resolution_too_high',
  'bit_depth_not_supported',
  'hdr_not_supported',
  'audio_codec_not_supported',
  'audio_channels_too_many',
  'audio_track_switch_unsupported',
  'subtitle_burn_in',
  'bitrate_too_high',
  'keyframe_index_missing',
  'forced_by_client',
  'colors_may_be_wrong',
] as const

/**
 * One plain sentence per reason. The session doesn't carry the codec or
 * the channel count, so those sentences don't name them. A remote stream
 * lowered by the admin's limit says so with the limit.
 * Keys are spelled out so the i18n extractor sees each of them.
 */
export function reasonText(
  t: TFunction,
  code: string,
  o: { remote?: boolean; limitMbps?: number },
): string {
  switch (code) {
    case 'container_not_supported':
      return t('reason.container_not_supported')
    case 'video_codec_not_supported':
      return t('reason.video_codec_not_supported')
    case 'video_profile_not_supported':
      return t('reason.video_profile_not_supported')
    case 'resolution_too_high':
      return t('reason.resolution_too_high')
    case 'bit_depth_not_supported':
      return t('reason.bit_depth_not_supported')
    case 'hdr_not_supported':
      return t('reason.hdr_not_supported')
    case 'audio_codec_not_supported':
      return t('reason.audio_codec_not_supported')
    case 'audio_channels_too_many':
      return t('reason.audio_channels_too_many')
    case 'audio_track_switch_unsupported':
      return t('reason.audio_track_switch_unsupported')
    case 'subtitle_burn_in':
      return t('reason.subtitle_burn_in')
    case 'bitrate_too_high':
      return o.remote && o.limitMbps
        ? t('reason.bitrate_too_high.remote_limit', { limit: o.limitMbps })
        : t('reason.bitrate_too_high.connection')
    case 'keyframe_index_missing':
      return t('reason.keyframe_index_missing')
    case 'forced_by_client':
      return t('reason.forced_by_client')
    case 'colors_may_be_wrong':
      return t('reason.colors_may_be_wrong')
    default:
      return t('reason.other')
  }
}

export function methodText(t: TFunction, method: string): string {
  switch (method) {
    case 'direct_play':
      return t('playing.method.direct_play')
    case 'direct_stream':
      return t('playing.method.direct_stream')
    default:
      return t('playing.method.transcode')
  }
}
