import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { Button, FolderPicker, TextField, type FolderEntry, type FolderStatus } from '../ui'

type Browse = components['schemas']['BrowseResult']

/** Over this many sub-folders the picker gets a filter (F007 R8). */
export const filterOver = 20

/**
 * The server folder picker as a sheet (phone) or side panel (desktop),
 * browsing /fs/browse one level at a time (F007 R8, R9).
 */
export function PickerSheet({
  serverName,
  onUse,
  onClose,
  onLost,
}: {
  serverName: string
  onUse: (path: string) => void
  onClose: () => void
  /** The server stopped answering. */
  onLost: () => void
}) {
  const { t } = useTranslation()
  const [dir, setDir] = useState<Browse>()
  const [notFound, setNotFound] = useState(false)
  const [expanded, setExpanded] = useState<string>()
  const [status, setStatus] = useState<Record<string, FolderStatus>>({})
  const [query, setQuery] = useState('')
  const [typing, setTyping] = useState(false)
  const [typed, setTyped] = useState('')
  const desktop = typeof matchMedia === 'function' && matchMedia('(min-width: 1024px)').matches

  const browse = useCallback(
    async (path?: string) => {
      try {
        const { data, response } = await api.GET('/fs/browse', {
          params: { query: path ? { path } : {} },
        })
        if (data) {
          setDir(data)
          setNotFound(false)
          setQuery('')
          setExpanded(undefined)
          return data
        }
        if (response.status === 404) setNotFound(true)
      } catch {
        onLost()
      }
      return undefined
    },
    [onLost],
  )

  useEffect(() => {
    api
      .GET('/fs/browse', { params: { query: {} } })
      .then(({ data }) => data && setDir(data))
      .catch(onLost)
  }, [onLost])

  // "Check again": list the parent again and see whether it's readable now.
  async function check(entry: FolderEntry) {
    setStatus((s) => ({ ...s, [entry.path]: 'checking' }))
    const again = await browse(dir?.path || undefined)
    const now = again?.entries.find((e) => e.path === entry.path)
    setStatus((s) => ({ ...s, [entry.path]: now?.readable ? 'fixed' : 'noAccess' }))
    setExpanded(now?.readable ? undefined : entry.path)
  }

  const atRoots = !!dir && dir.path === ''
  const all: FolderEntry[] = (dir?.entries ?? []).map((e) => ({
    name: e.name,
    path: e.path,
    status: status[e.path] ?? (e.readable ? 'ok' : 'noAccess'),
    expanded: expanded === e.path,
  }))
  const shown = query ? all.filter((e) => e.name.toLowerCase().includes(query.toLowerCase())) : all

  function submitPath(e: FormEvent) {
    e.preventDefault()
    if (typed.startsWith('/')) void browse(typed).then((d) => d && setTyping(false))
  }

  return (
    <div
      className="ob__picker"
      role="dialog"
      aria-modal="true"
      aria-label={t('picker.title', { serverName })}
    >
      <FolderPicker
        serverName={serverName}
        layout={desktop ? 'panel' : 'sheet'}
        path={dir?.path ?? ''}
        atRoots={atRoots}
        entries={shown}
        filter={
          all.length > filterOver
            ? { value: query, total: all.length, onChange: setQuery }
            : undefined
        }
        onOpen={(e) => void browse(e.path)}
        onToggle={(e) => setExpanded(expanded === e.path ? undefined : e.path)}
        onCheck={(e) => void check(e)}
        onParent={dir?.parent != null ? () => void browse(dir.parent ?? undefined) : undefined}
        onUse={() => dir?.path && onUse(dir.path)}
        onClose={onClose}
        onTypePath={() => setTyping(true)}
      />
      {typing && (
        <form className="ob__picker-path" onSubmit={submitPath}>
          <TextField
            label={t('picker.typePath')}
            value={typed}
            onValue={setTyped}
            autoCapitalize="none"
            spellCheck={false}
            error={notFound ? t('errors.folder_not_found') : undefined}
            style={{ fontFamily: 'var(--font-code)' }}
          />
          <Button type="submit" variant="secondary" style={{ alignSelf: 'flex-end' }}>
            {t('picker.open')}
          </Button>
        </form>
      )}
    </div>
  )
}
