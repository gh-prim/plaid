import { useEffect, useRef, useState, type PointerEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type Profile } from '../api/client'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { Button, Icon } from '../ui'
import './avatar.css'

/** The square the server keeps (F009): 512 px. */
const OUT = 512
/** The crop window on screen. */
const VIEW = 280

/**
 * A photo, framed square (board Avatar02): drag to place it, zoom with the
 * slider, Use sends a 512 px JPEG. The server keeps none of the photo's
 * metadata.
 */
export function PhotoCrop({
  profile,
  onDone,
  onCancel,
}: {
  profile: Profile
  onDone: (p: Profile) => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const input = useRef<HTMLInputElement>(null)
  const [img, setImg] = useState<HTMLImageElement>()
  const [zoom, setZoom] = useState(1)
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const drag = useRef<{ x: number; y: number; ox: number; oy: number }>(undefined)
  const [bad, setBad] = useState(false)
  const [saving, setSaving] = useState(false)
  const [fail, setFail] = useState<Failure>()

  useEffect(() => {
    input.current?.click()
  }, [])

  function choose(file: File | undefined) {
    if (!file) return
    setBad(false)
    const url = URL.createObjectURL(file)
    const image = new Image()
    image.onload = () => {
      setImg(image)
      setZoom(1)
      setOffset({ x: 0, y: 0 })
    }
    image.onerror = () => setBad(true)
    image.src = url
  }

  // The image covers the window at zoom 1; offsets are in window pixels.
  const cover = img ? VIEW / Math.min(img.naturalWidth, img.naturalHeight) : 1
  const scale = cover * zoom
  const w = img ? img.naturalWidth * scale : 0
  const h = img ? img.naturalHeight * scale : 0
  const clamp = (o: { x: number; y: number }) => ({
    x: Math.min(Math.max(o.x, -(w - VIEW) / 2), (w - VIEW) / 2),
    y: Math.min(Math.max(o.y, -(h - VIEW) / 2), (h - VIEW) / 2),
  })
  const pos = clamp(offset)

  function down(e: PointerEvent) {
    e.currentTarget.setPointerCapture(e.pointerId)
    drag.current = { x: e.clientX, y: e.clientY, ox: pos.x, oy: pos.y }
  }
  function move(e: PointerEvent) {
    const d = drag.current
    if (d) setOffset({ x: d.ox + e.clientX - d.x, y: d.oy + e.clientY - d.y })
  }

  async function use() {
    if (!img) return
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = OUT
    const ctx = canvas.getContext('2d')!
    // The window's square, in source pixels.
    const side = VIEW / scale
    const sx = (img.naturalWidth - side) / 2 - pos.x / scale
    const sy = (img.naturalHeight - side) / 2 - pos.y / scale
    ctx.drawImage(img, sx, sy, side, side, 0, 0, OUT, OUT)
    const blob = await new Promise<Blob | null>((r) => canvas.toBlob(r, 'image/jpeg', 0.9))
    if (!blob) return setBad(true)
    setSaving(true)
    setFail(undefined)
    const { data, error, response } = await api.PUT('/me/profiles/{profileId}/avatar/photo', {
      params: { path: { profileId: profile.id } },
      body: blob as unknown as string,
      bodySerializer: (b: unknown) => b as Blob,
      headers: { 'Content-Type': 'image/jpeg' },
    })
    setSaving(false)
    if (error) return setFail(failure(error, response))
    onDone(data)
  }

  return (
    <div className="av">
      <header className="av__top">
        <button
          type="button"
          className="ob-glassbtn pl-focusable av__back"
          aria-label={t('common.back')}
          onClick={onCancel}
        >
          <Icon name="chevronLeft" size={20} />
        </button>
        <h1 className="av__title">{t('avatar.crop.title')}</h1>
        <Button size="sm" loading={saving} disabled={!img} onClick={() => void use()}>
          {t('avatar.crop.use')}
        </Button>
      </header>
      <input
        ref={input}
        className="av__file"
        type="file"
        accept="image/jpeg,image/png,image/webp"
        onChange={(e) => choose(e.target.files?.[0])}
      />
      <div className="av__crop" style={{ width: VIEW, height: VIEW }}>
        {img && (
          <img
            src={img.src}
            alt=""
            draggable={false}
            onPointerDown={down}
            onPointerMove={move}
            onPointerUp={() => (drag.current = undefined)}
            style={{
              width: w,
              height: h,
              transform: `translate(${(VIEW - w) / 2 + pos.x}px, ${(VIEW - h) / 2 + pos.y}px)`,
            }}
          />
        )}
      </div>
      {img && (
        <label className="av__zoom">
          <span>{t('avatar.crop.zoom')}</span>
          <input
            type="range"
            min={1}
            max={4}
            step={0.01}
            value={zoom}
            onChange={(e) => setZoom(Number(e.target.value))}
          />
        </label>
      )}
      <p className="av__hint">{t('avatar.crop.help')}</p>
      <Button variant="secondary" onClick={() => input.current?.click()}>
        {t('avatar.crop.choose')}
      </Button>
      {(bad || fail) && (
        <p role="alert" className="av__error">
          {bad ? t('avatar.crop.error') : errorMessage(t, fail!)}
        </p>
      )}
    </div>
  )
}
