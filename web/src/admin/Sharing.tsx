import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { errorMessage, failure, type Failure } from '../auth/errors'
import { Avatar, Button, Dialog, EmptyState, Mark, TextField } from '../ui'
import { useAdmin } from './AdminLayout'
import { Group, Loading, Row, Section, Switch } from './v2'

type AdminUser = components['schemas']['AdminUser']
type SharingGroup = components['schemas']['Group']
type Invitation = components['schemas']['Invitation']
type Library = components['schemas']['Library']
type Rights = components['schemas']['Rights']
type RightsOverride = components['schemas']['RightsOverride']
type MaxQuality = components['schemas']['MaxQuality']
type PartyRight = components['schemas']['PartyRight']

const qualities: MaxQuality[] = ['original', '2160p', '1080p', '720p', '480p']
const parties: PartyRight[] = ['none', 'join', 'start']
const streamCounts = [1, 2, 3, 4, 5, 10]
const inviteDays = [1, 7, 14, 30]

type SharingData = {
  groups: SharingGroup[]
  libraries: Library[]
  users: AdminUser[]
  invitations: Invitation[]
}

async function fetchSharing(): Promise<SharingData> {
  const [g, l, u, i] = await Promise.all([
    api.GET('/groups'),
    api.GET('/libraries'),
    api.GET('/users'),
    api.GET('/invitations'),
  ])
  return {
    groups: g.data ?? [],
    libraries: l.data ?? [],
    users: u.data ?? [],
    invitations: i.data ?? [],
  }
}

/** Groups, libraries, people and pending invitations: everything sharing needs. */
export function useSharing() {
  const [data, setData] = useState<SharingData>()
  const load = useCallback(() => fetchSharing().then(setData), [])
  useEffect(() => {
    void fetchSharing().then(setData)
  }, [])
  return { data, reload: load }
}

/** The faces of a group, three at most then "+n" (board Share02). */
export function Faces({ ids, users }: { ids: string[]; users: AdminUser[] }) {
  const { t } = useTranslation()
  const people = ids.map((id) => users.find((u) => u.id === id)).filter((u) => u !== undefined)
  return (
    <span className="pl pl-avatar-stack" style={{ '--pl-s': '30px' } as React.CSSProperties}>
      {people.slice(0, 3).map((u) => (
        <Avatar
          key={u.id}
          name={u.displayName}
          color={u.avatarColor + 1}
          picture={u.avatar.url}
          size={30}
        />
      ))}
      {people.length > 3 && (
        <span className="pl-avatar-stack__more">{t('sharing.more', { n: people.length - 3 })}</span>
      )}
    </span>
  )
}

/** One line of what a group gives: libraries, quality, streams. */
export function useRightsSummary() {
  const { t } = useTranslation()
  return (g: SharingGroup, libraries: Library[]) => {
    if (g.household) return t('sharing.group.everything')
    const names = g.libraryIds
      .map((id) => libraries.find((l) => l.id === id)?.name)
      .filter(Boolean)
      .join(', ')
    return [
      names || t('sharing.group.noLibrary'),
      t(`sharing.quality.${g.rights.maxQuality}`),
      g.rights.maxStreams == null
        ? t('sharing.streams.unlimited')
        : t('sharing.streams.count', { count: g.rights.maxStreams }),
    ].join(' · ')
  }
}

/** The groups in People: one row each, then New group (board Share01). */
export function GroupsGroup({
  groups,
  users,
  libraries,
  onChange,
}: {
  groups: SharingGroup[]
  users: AdminUser[]
  libraries: Library[]
  onChange: () => void
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const summary = useRightsSummary()
  const [naming, setNaming] = useState(false)
  return (
    <Group title={t('sharing.groups.title')} foot={t('sharing.groups.foot')}>
      {groups.map((g) => (
        <Row
          key={g.id}
          to={`/admin/people/groups/${g.id}`}
          avatar={<Faces ids={g.memberIds} users={users} />}
          label={g.name}
          sub={summary(g, libraries)}
          value={t('admin.people.count', { count: g.memberIds.length })}
        />
      ))}
      <Row label={t('sharing.groups.new')} onClick={() => setNaming(true)} />
      {naming && (
        <NameDialog
          title={t('sharing.groups.new')}
          onClose={() => setNaming(false)}
          onSave={async (name) => {
            const { data, error, response } = await api.POST('/groups', { body: { name } })
            if (error) return failure(error, response)
            setNaming(false)
            onChange()
            void navigate(`/admin/people/groups/${data.id}`)
          }}
        />
      )}
    </Group>
  )
}

export function NameDialog({
  title,
  initial = '',
  onClose,
  onSave,
}: {
  title: string
  initial?: string
  onClose: () => void
  onSave: (name: string) => Promise<Failure | void>
}) {
  const { t } = useTranslation()
  const [name, setName] = useState(initial)
  const [fail, setFail] = useState<Failure>()
  const [saving, setSaving] = useState(false)
  async function submit(e?: FormEvent) {
    e?.preventDefault()
    if (!name.trim()) return
    setSaving(true)
    const f = await onSave(name.trim())
    setSaving(false)
    if (f) setFail(f)
  }
  return (
    <Dialog
      title={title}
      actions={
        <>
          <Button variant="ghost" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button loading={saving} disabled={!name.trim()} onClick={() => void submit()}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <form onSubmit={(e) => void submit(e)} noValidate>
        <TextField
          label={t('sharing.group.name')}
          value={name}
          onValue={setName}
          error={
            fail
              ? fail.error?.code === 'validation_failed'
                ? t('sharing.group.nameTaken')
                : errorMessage(t, fail)
              : undefined
          }
        />
      </form>
    </Dialog>
  )
}

/** Pending invitations, and Invite (board Share03). */
export function InvitationsGroup({
  invitations,
  groups,
  onChange,
}: {
  invitations: Invitation[]
  groups: SharingGroup[]
  onChange: () => void
}) {
  const { t, i18n } = useTranslation()
  const [inviting, setInviting] = useState(false)
  const date = new Intl.DateTimeFormat(i18n.resolvedLanguage, { day: 'numeric', month: 'short' })
  return (
    <Group
      title={t('sharing.invitations.title', { count: invitations.length })}
      foot={t('sharing.invitations.foot')}
    >
      {invitations.map((inv) => (
        <Row
          key={inv.id}
          label={inv.name}
          sub={[
            inv.groupIds
              .map((id) => groups.find((g) => g.id === id)?.name)
              .filter(Boolean)
              .join(', '),
            t('sharing.invitations.expires', { date: date.format(new Date(inv.expiresAt)) }),
          ]
            .filter(Boolean)
            .join(' · ')}
          control={
            <Button
              variant="secondary"
              size="sm"
              onClick={() =>
                void api
                  .DELETE('/invitations/{invitationId}', {
                    params: { path: { invitationId: inv.id } },
                  })
                  .then(onChange)
              }
            >
              {t('sharing.invitations.cancel')}
            </Button>
          }
        />
      ))}
      <Row label={t('sharing.invite.title')} onClick={() => setInviting(true)} />
      {inviting && (
        <InviteDialog
          groups={groups}
          onClose={() => {
            setInviting(false)
            onChange()
          }}
        />
      )}
    </Group>
  )
}

/** Invite someone into groups: a name for you, groups, an expiry, then the link. */
export function InviteDialog({
  groups,
  groupId,
  onClose,
}: {
  groups: SharingGroup[]
  groupId?: string
  onClose: () => void
}) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [picked, setPicked] = useState<string[]>(groupId ? [groupId] : [])
  const [days, setDays] = useState(7)
  const [link, setLink] = useState<string>()
  const [copied, setCopied] = useState(false)
  const [fail, setFail] = useState<Failure>()
  const [saving, setSaving] = useState(false)

  async function create() {
    setSaving(true)
    const { data, error, response } = await api.POST('/invitations', {
      body: { name: name.trim(), groupIds: picked, expiresInDays: days },
    })
    setSaving(false)
    if (error) return setFail(failure(error, response))
    setLink(`${window.location.origin}/invite/${data.token}`)
  }

  if (link)
    return (
      <Dialog
        title={t('sharing.invite.ready', { name: name.trim() })}
        actions={
          <>
            <Button
              variant="secondary"
              onClick={() =>
                void navigator.clipboard.writeText(link).then(() => setCopied(true), undefined)
              }
            >
              {copied ? t('sharing.invite.copied') : t('sharing.invite.copy')}
            </Button>
            <Button onClick={onClose}>{t('common.done')}</Button>
          </>
        }
      >
        <code className="adm__path sh__link">{link}</code>
        <p className="adm__text">{t('sharing.invite.once', { count: days })}</p>
      </Dialog>
    )

  return (
    <Dialog
      title={t('sharing.invite.title')}
      actions={
        <>
          <Button variant="ghost" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button
            loading={saving}
            disabled={!name.trim() || picked.length === 0}
            onClick={() => void create()}
          >
            {t('sharing.invite.create')}
          </Button>
        </>
      }
    >
      <p className="adm__text">{t('sharing.invite.lead')}</p>
      <TextField label={t('sharing.invite.name')} value={name} onValue={setName} />
      <fieldset className="ob__fieldset">
        <legend className="ob__legend">{t('sharing.invite.groups')}</legend>
        <ul className="sh__checks">
          {groups.map((g) => (
            <li key={g.id}>
              <label className="sh__check">
                <input
                  type="checkbox"
                  checked={picked.includes(g.id)}
                  onChange={(e) =>
                    setPicked((p) =>
                      e.target.checked ? [...p, g.id] : p.filter((x) => x !== g.id),
                    )
                  }
                />
                <span>{g.name}</span>
              </label>
            </li>
          ))}
        </ul>
      </fieldset>
      <label className="sh__inline">
        <span>{t('sharing.invite.expires')}</span>
        <select
          className="adm__select"
          value={days}
          onChange={(e) => setDays(Number(e.target.value))}
        >
          {inviteDays.map((d) => (
            <option key={d} value={d}>
              {t('sharing.invite.days', { count: d })}
            </option>
          ))}
        </select>
      </label>
      {fail && (
        <p role="alert" className="adm__text pl-tone--error">
          <Mark tone="error" />
          {errorMessage(t, fail)}
        </p>
      )}
    </Dialog>
  )
}

/** The rights rows of a group, or of one person on top of their groups. */
function RightsRows({
  rights,
  inherited,
  onChange,
  busy,
}: {
  rights: Rights | RightsOverride
  /** For one person: what their groups give; the selects then offer "From the group". */
  inherited?: Rights
  onChange: (r: Partial<RightsOverride>) => void
  busy?: boolean
}) {
  const { t } = useTranslation()
  const fromGroup = (v: string) => (inherited ? t('sharing.fromGroup', { value: v }) : v)
  const quality = rights.maxQuality ?? ''
  const streams =
    inherited && 'unlimitedStreams' in rights && rights.unlimitedStreams
      ? 'unlimited'
      : rights.maxStreams == null
        ? inherited
          ? ''
          : 'unlimited'
        : String(rights.maxStreams)
  const party = rights.party ?? ''
  const streamLabel = (n: number | null | undefined) =>
    n == null ? t('sharing.streams.unlimited') : t('sharing.streams.count', { count: n })
  return (
    <>
      <Row
        label={t('sharing.quality.label')}
        control={
          <select
            className="adm__select"
            aria-label={t('sharing.quality.label')}
            disabled={busy}
            value={quality}
            onChange={(e) =>
              onChange({ maxQuality: (e.target.value || null) as MaxQuality | null })
            }
          >
            {inherited && (
              <option value="">{fromGroup(t(`sharing.quality.${inherited.maxQuality}`))}</option>
            )}
            {qualities.map((q) => (
              <option key={q} value={q}>
                {t(`sharing.quality.${q}`)}
              </option>
            ))}
          </select>
        }
      />
      <Row
        label={t('sharing.streams.label')}
        sub={t('sharing.streams.help')}
        control={
          <select
            className="adm__select"
            aria-label={t('sharing.streams.label')}
            disabled={busy}
            value={streams}
            onChange={(e) => {
              const v = e.target.value
              if (inherited)
                onChange({
                  maxStreams: v && v !== 'unlimited' ? Number(v) : null,
                  unlimitedStreams: v === 'unlimited' ? true : null,
                })
              else onChange({ maxStreams: v === 'unlimited' ? null : Number(v) })
            }}
          >
            {inherited && <option value="">{fromGroup(streamLabel(inherited.maxStreams))}</option>}
            {streamCounts.map((n) => (
              <option key={n} value={n}>
                {streamLabel(n)}
              </option>
            ))}
            <option value="unlimited">{streamLabel(null)}</option>
          </select>
        }
      />
      <Row
        label={t('sharing.party.label')}
        control={
          <select
            className="adm__select"
            aria-label={t('sharing.party.label')}
            disabled={busy}
            value={party}
            onChange={(e) => onChange({ party: (e.target.value || null) as PartyRight | null })}
          >
            {inherited && (
              <option value="">{fromGroup(t(`sharing.party.${inherited.party}`))}</option>
            )}
            {parties.map((p) => (
              <option key={p} value={p}>
                {t(`sharing.party.${p}`)}
              </option>
            ))}
          </select>
        }
      />
      <Row
        label={t('sharing.downloads')}
        sub={t('sharing.soon')}
        control={
          <Switch
            label={t('sharing.downloads')}
            on={(rights.downloads ?? inherited?.downloads) === true}
            disabled={busy || (inherited?.downloads ?? false)}
            onChange={(on) => onChange({ downloads: inherited && !on ? null : on })}
          />
        }
      />
      <Row
        label={t('sharing.profiles')}
        sub={t('sharing.profilesHelp')}
        control={
          <Switch
            label={t('sharing.profiles')}
            on={(rights.canCreateProfiles ?? inherited?.canCreateProfiles) === true}
            disabled={busy || (inherited?.canCreateProfiles ?? false)}
            onChange={(on) => onChange({ canCreateProfiles: inherited && !on ? null : on })}
          />
        }
      />
    </>
  )
}

/** A library's line: its name, its size, and why it may be off. */
function libraryCount(t: ReturnType<typeof useTranslation>['t'], l: Library) {
  return l.type === 'shows'
    ? t('sharing.library.shows', { count: l.counts.shows })
    : t('sharing.library.movies', { count: l.counts.movies })
}

/** One group: who's in it and what they get (board Share02). */
export function GroupPage() {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { toast } = useAdmin()
  const { data, reload } = useSharing()
  const [busy, setBusy] = useState<string>()
  const [fail, setFail] = useState<Failure>()
  const [inviting, setInviting] = useState(false)
  const [renaming, setRenaming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const back = { to: '/admin/people', label: t('people.title') }
  const group = data?.groups.find((g) => g.id === id)

  if (!data || !group)
    return (
      <Section title={t('sharing.groups.title')} back={back} glyph="people">
        {data ? <EmptyState title={t('sharing.group.gone')} /> : <Loading />}
      </Section>
    )

  async function patch(key: string, body: components['schemas']['UpdateGroupRequest']) {
    setBusy(key)
    setFail(undefined)
    const { error, response } = await api.PATCH('/groups/{groupId}', {
      params: { path: { groupId: id } },
      body,
    })
    setBusy(undefined)
    if (error) return setFail(failure(error, response))
    await reload()
  }

  const g = group
  const exceptions = data.users.filter(
    (u) => g.memberIds.includes(u.id) && u.extraLibraryIds.length > 0,
  ).length
  return (
    <Section
      title={g.name}
      back={back}
      glyph="people"
      paneTitle={g.name}
      paneSub={
        <>
          {t('sharing.group.lead', { count: g.memberIds.length })}
          {exceptions > 0 && (
            <>
              <br />
              {t('sharing.group.exceptions', { count: exceptions })}
            </>
          )}
        </>
      }
    >
      <Group>
        <Row
          avatar={<Faces ids={g.memberIds} users={data.users} />}
          label={t('admin.people.count', { count: g.memberIds.length })}
          sub={g.memberIds
            .map((m) => data.users.find((u) => u.id === m)?.displayName)
            .filter(Boolean)
            .join(', ')}
        />
        <Row label={t('sharing.group.invite')} onClick={() => setInviting(true)} />
        <Row label={t('sharing.group.rename')} value={g.name} onClick={() => setRenaming(true)} />
      </Group>

      <Group
        title={t('sharing.libraries.title')}
        foot={g.household ? t('sharing.group.householdFoot') : undefined}
      >
        <Row
          label={t('sharing.group.household')}
          sub={t('sharing.group.householdHelp')}
          busy={busy === 'household'}
          control={
            <Switch
              label={t('sharing.group.household')}
              on={g.household}
              disabled={busy === 'household'}
              onChange={(on) => void patch('household', { household: on })}
            />
          }
        />
        {!g.household &&
          data.libraries.map((l) => (
            <Row
              key={l.id}
              label={l.name}
              sub={l.householdOnly ? t('sharing.library.householdOnly') : libraryCount(t, l)}
              muted={l.householdOnly}
              busy={busy === l.id}
              control={
                <Switch
                  label={l.name}
                  on={g.libraryIds.includes(l.id)}
                  disabled={l.householdOnly || busy === l.id}
                  onChange={(on) =>
                    void patch(l.id, {
                      libraryIds: on
                        ? [...g.libraryIds, l.id]
                        : g.libraryIds.filter((x) => x !== l.id),
                    })
                  }
                />
              }
            />
          ))}
      </Group>

      <Group title={t('sharing.rights.title')}>
        <RightsRows
          rights={g.rights}
          busy={busy === 'rights'}
          onChange={(r) => void patch('rights', { rights: { ...g.rights, ...r } as Rights })}
        />
      </Group>
      {fail && (
        <p role="alert" className="adm__text pl-tone--error adm-sec__alert">
          <Mark tone="error" />
          {errorMessage(t, fail)}
        </p>
      )}

      <Group foot={t('sharing.group.deleteFoot')}>
        <Row danger label={t('sharing.group.delete')} onClick={() => setDeleting(true)} />
      </Group>

      {inviting && (
        <InviteDialog groups={data.groups} groupId={g.id} onClose={() => setInviting(false)} />
      )}
      {renaming && (
        <NameDialog
          title={t('sharing.group.rename')}
          initial={g.name}
          onClose={() => setRenaming(false)}
          onSave={async (name) => {
            const { error, response } = await api.PATCH('/groups/{groupId}', {
              params: { path: { groupId: id } },
              body: { name },
            })
            if (error) return failure(error, response)
            setRenaming(false)
            await reload()
          }}
        />
      )}
      {deleting && (
        <Dialog
          title={t('sharing.group.deleteConfirm', { name: g.name })}
          actions={
            <>
              <Button variant="ghost" onClick={() => setDeleting(false)}>
                {t('common.cancel')}
              </Button>
              <Button
                onClick={() =>
                  void (async () => {
                    const { error, response } = await api.DELETE('/groups/{groupId}', {
                      params: { path: { groupId: id } },
                    })
                    setDeleting(false)
                    if (error) return setFail(failure(error, response))
                    toast(t('sharing.group.deleted', { name: g.name }))
                    await navigate('/admin/people')
                  })()
                }
              >
                {t('sharing.group.delete')}
              </Button>
            </>
          }
        />
      )}
    </Section>
  )
}

/** One person's groups and exceptions (board Share04). */
export function PersonSharing({
  user,
  onChange,
}: {
  user: AdminUser
  onChange: () => Promise<void>
}) {
  const { t } = useTranslation()
  const { data, reload } = useSharing()
  const [busy, setBusy] = useState<string>()
  const [fail, setFail] = useState<Failure>()
  if (user.role === 'admin') return null
  if (!data)
    return (
      <Group title={t('sharing.person.groups')}>
        <li>
          <Loading />
        </li>
      </Group>
    )

  async function update(key: string, body: components['schemas']['UpdateUserRequest']) {
    setBusy(key)
    setFail(undefined)
    const { error, response } = await api.PATCH('/users/{userId}', {
      params: { path: { userId: user.id } },
      body,
    })
    setBusy(undefined)
    if (error) return setFail(failure(error, response))
    await Promise.all([reload(), onChange()])
  }

  // What their groups give, without their own exceptions.
  const groups = data.groups.filter((g) => user.groupIds.includes(g.id))
  const fromGroups = groups.reduce<Rights | undefined>((acc, g) => {
    if (!acc) return g.rights
    const best = (a: string, b: string, order: string[]) =>
      order.indexOf(a) <= order.indexOf(b) ? a : b
    return {
      maxQuality: best(acc.maxQuality, g.rights.maxQuality, qualities) as MaxQuality,
      maxStreams:
        acc.maxStreams == null || g.rights.maxStreams == null
          ? null
          : Math.max(acc.maxStreams, g.rights.maxStreams),
      downloads: acc.downloads || g.rights.downloads,
      party: best(acc.party, g.rights.party, [...parties].reverse()) as PartyRight,
      canCreateProfiles: acc.canCreateProfiles || g.rights.canCreateProfiles,
    }
  }, undefined) ?? {
    maxQuality: '480p',
    maxStreams: 1,
    downloads: false,
    party: 'none',
    canCreateProfiles: false,
  }
  const household = groups.some((g) => g.household)
  const viaGroups = new Set(groups.flatMap((g) => g.libraryIds))

  return (
    <>
      <Group title={t('sharing.person.groups')} foot={t('sharing.person.groupsFoot')}>
        {data.groups.map((g) => (
          <Row
            key={g.id}
            label={g.name}
            busy={busy === g.id}
            control={
              <Switch
                label={g.name}
                on={user.groupIds.includes(g.id)}
                disabled={busy === g.id}
                onChange={(on) =>
                  void update(g.id, {
                    groupIds: on
                      ? [...user.groupIds, g.id]
                      : user.groupIds.filter((x) => x !== g.id),
                  })
                }
              />
            }
          />
        ))}
        {!data.groups.length && <Row muted label={t('sharing.person.noGroups')} />}
      </Group>

      <Group
        title={t('sharing.person.just', { name: user.displayName })}
        foot={t('sharing.person.justFoot')}
      >
        {!household &&
          data.libraries.map((l) => (
            <Row
              key={l.id}
              label={l.name}
              sub={
                l.householdOnly
                  ? t('sharing.library.householdOnly')
                  : viaGroups.has(l.id)
                    ? t('sharing.person.viaGroup')
                    : undefined
              }
              muted={l.householdOnly}
              control={
                <Switch
                  label={l.name}
                  on={viaGroups.has(l.id) || user.extraLibraryIds.includes(l.id)}
                  disabled={l.householdOnly || viaGroups.has(l.id) || busy === l.id}
                  onChange={(on) =>
                    void update(l.id, {
                      extraLibraryIds: on
                        ? [...user.extraLibraryIds, l.id]
                        : user.extraLibraryIds.filter((x) => x !== l.id),
                    })
                  }
                />
              }
            />
          ))}
        <RightsRows
          rights={user.extraRights}
          inherited={fromGroups}
          busy={busy === 'rights'}
          onChange={(r) => void update('rights', { extraRights: { ...user.extraRights, ...r } })}
        />
      </Group>
      {fail && (
        <p role="alert" className="adm__text pl-tone--error adm-sec__alert">
          <Mark tone="error" />
          {errorMessage(t, fail)}
        </p>
      )}
    </>
  )
}

/** Who can see a library: its groups, the people on their own (board Share05). */
export function LibrarySharing({ library, onChange }: { library: Library; onChange: () => void }) {
  const { t } = useTranslation()
  const { data, reload } = useSharing()
  const summary = useRightsSummary()
  const [busy, setBusy] = useState(false)
  if (!data)
    return (
      <Group title={t('sharing.library.who')}>
        <li>
          <Loading />
        </li>
      </Group>
    )
  const groups = data.groups.filter((g) => g.household || g.libraryIds.includes(library.id))
  const own = data.users.filter((u) => u.role !== 'admin' && u.extraLibraryIds.includes(library.id))
  return (
    <>
      <Group title={t('sharing.library.who')} foot={t('sharing.library.adminsFoot')}>
        {groups.map((g) => (
          <Row
            key={g.id}
            to={`/admin/people/groups/${g.id}`}
            label={g.name}
            sub={summary(g, data.libraries)}
            value={t('admin.people.count', { count: g.memberIds.length })}
          />
        ))}
        {!groups.length && <Row muted label={t('sharing.library.nobody')} />}
        {own.map((u) => (
          <Row
            key={u.id}
            to={`/admin/people/${u.id}`}
            avatar={
              <Avatar
                name={u.displayName}
                color={u.avatarColor + 1}
                picture={u.avatar.url}
                size={30}
              />
            }
            label={u.displayName}
            sub={t('sharing.library.onTop', {
              groups: u.groupIds
                .map((id) => data.groups.find((g) => g.id === id)?.name)
                .filter(Boolean)
                .join(', '),
            })}
          />
        ))}
      </Group>
      <Group foot={t('sharing.library.householdOnlyFoot')}>
        <Row
          label={t('sharing.library.householdOnlyLabel')}
          busy={busy}
          control={
            <Switch
              label={t('sharing.library.householdOnlyLabel')}
              on={library.householdOnly}
              disabled={busy}
              onChange={(on) =>
                void (async () => {
                  setBusy(true)
                  await api.PATCH('/libraries/{libraryId}', {
                    params: { path: { libraryId: library.id } },
                    body: { householdOnly: on },
                  })
                  setBusy(false)
                  await reload()
                  onChange()
                })()
              }
            />
          }
        />
      </Group>
    </>
  )
}
