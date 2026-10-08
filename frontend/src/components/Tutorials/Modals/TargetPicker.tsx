import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Building2, Globe, Layers, Network, Shield, User, UserCheck, UserCog, Users, X } from 'lucide-react'
import { tutorialService } from '../../../services/api'
import RecipientSelector from '../../../pages/Email/RecipientSelector'
import { DEFAULT_ROLES, isEmptyTarget } from '../../../types'
import type { TargetRole, TutorialAudience, TutorialTarget } from '../../../types'
import { audienceForTarget } from '../audienceLabel'
import styles from './TargetPicker.module.css'

interface TargetPickerProps {
  value: TutorialTarget
  onChange: (target: TutorialTarget, audience: TutorialAudience) => void
}

type Tab = 'empresas' | 'paises' | 'grupos'

const TABS: { value: Tab; label: string; icon: typeof Building2 }[] = [
  { value: 'empresas', label: 'Empresas', icon: Building2 },
  { value: 'paises', label: 'Países', icon: Globe },
  { value: 'grupos', label: 'Grupos', icon: Users },
]

/**
 * Los perfiles, en el orden en que se piensan. Managers y supervisores son
 * profesionales pero van aparte: «Profesionales» es quien no tiene equipo a
 * cargo, así que se puede escribir solo a los managers sin que les llegue al
 * resto, o a todos marcando los tres.
 */
const ROLES: { value: TargetRole; label: string; hint: string; icon: typeof User }[] = [
  { value: 'empresa', label: 'Empresas', hint: 'Cuentas de empresa', icon: Building2 },
  { value: 'profesional', label: 'Profesionales', hint: 'Sin equipo a cargo', icon: User },
  { value: 'manager', label: 'Managers', hint: 'Con equipo a cargo', icon: UserCog },
  { value: 'supervisor', label: 'Supervisores', hint: 'Managers de managers', icon: Network },
  { value: 'superadmin', label: 'Superadmins', hint: 'Equipo de Oberstaff', icon: Shield },
]

// Lo que se puede elegir por personas: quien tiene el módulo de Novedades.
// CS y analistas de IT no lo tienen, así que no se ofrecen.
const PEOPLE_ROLE_FILTERS = ['all', 'profesional_activo', 'empleador', 'manager', 'supervisor', 'superadmin']
const isEligible = (u: { user_type: string; is_superadmin: boolean; is_active?: boolean }) =>
  u.is_active !== false && (u.is_superadmin || ['empleador', 'profesional', 'superadmin'].includes(u.user_type))

const STAT_LABELS: Record<string, string> = {
  empleador: 'empresas',
  profesional: 'profesionales',
  superadmin: 'superadmins',
}

/** Quita o agrega un elemento de una lista, sin repetirlo. */
function toggle<T>(list: T[], item: T): T[] {
  return list.includes(item) ? list.filter(i => i !== item) : [...list, item]
}

/**
 * Público de una novedad, en dos modos:
 *
 * - Por perfil: roles (empresas, profesionales, managers, supervisores,
 *   superadmins) y, si hace falta, acotado por empresa, país o grupo. Es una
 *   regla: quien entre después en ese perfil también la recibe.
 * - Personas concretas: una lista elegida a mano con los mismos filtros que
 *   Tools › Correos. Llega a esa gente y a nadie más.
 *
 * Lo importante no es el selector sino el contador: mientras se elige, se
 * consulta a cuánta gente llegaría.
 */
export function TargetPicker({ value, onChange }: TargetPickerProps) {
  const [tab, setTab] = useState<Tab>('empresas')
  const [search, setSearch] = useState('')
  const [expanded, setExpanded] = useState(!isEmptyTarget(value) && value.mode !== 'personas')
  const mode = value.mode === 'personas' ? 'personas' : 'perfil'
  const roles = value.roles ?? DEFAULT_ROLES

  const set = (next: TutorialTarget) => onChange(next, audienceForTarget(next))

  const { data: options } = useQuery({
    queryKey: ['tutorial-audience-options'],
    queryFn: () => tutorialService.getAudienceOptions(),
    staleTime: 5 * 60_000,
  })

  const { data: preview, isFetching: isPreviewing } = useQuery({
    queryKey: ['tutorial-audience-preview', value],
    queryFn: () => tutorialService.previewAudience(audienceForTarget(value), value),
    staleTime: 30_000,
  })

  const companies = options?.companies ?? []
  const countries = options?.countries ?? []
  const groups = options?.groups ?? []
  const roleCounts = options?.role_counts ?? {}

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase()
    if (tab === 'paises') {
      return countries.filter(c => c.toLowerCase().includes(needle))
    }
    const list = tab === 'empresas' ? companies : groups
    return list.filter(o => o.name.toLowerCase().includes(needle))
  }, [tab, search, companies, countries, groups])

  const chips = [
    ...value.company_ids.map(id => ({
      key: `c${id}`,
      label: companies.find(c => c.id === id)?.name ?? `Empresa ${id}`,
      remove: () => set({ ...value, company_ids: value.company_ids.filter(i => i !== id) }),
    })),
    ...value.countries.map(country => ({
      key: `p${country}`,
      label: country,
      remove: () => set({ ...value, countries: value.countries.filter(c => c !== country) }),
    })),
    ...value.group_ids.map(id => ({
      key: `g${id}`,
      label: groups.find(g => g.id === id)?.name ?? `Grupo ${id}`,
      remove: () => set({ ...value, group_ids: value.group_ids.filter(i => i !== id) }),
    })),
  ]

  // Cambiar de modo empieza los grupos de cero: en un modo suman gente y en
  // el otro la acotan, así que arrastrarlos cambiaría su sentido sin avisar.
  const switchMode = (next: 'perfil' | 'personas') => {
    if (next === mode) return
    setExpanded(false)
    set({ ...value, mode: next, roles, company_ids: [], countries: [], group_ids: [], managers_only: false })
  }

  // Al plegar los filtros se limpian: dejarlos puestos sin que se vean sería
  // publicar creyendo que llega a todo el perfil.
  const toggleExpanded = (open: boolean) => {
    setExpanded(open)
    if (!open && !isEmptyTarget(value)) {
      set({ ...value, company_ids: [], countries: [], group_ids: [], managers_only: false })
    }
  }

  const allRoles = ROLES.map(r => r.value)
  const everyone = DEFAULT_ROLES.every(r => roles.includes(r))

  return (
    <div className={styles['picker']}>
      <div className={styles['modes']} role="radiogroup" aria-label="Cómo elegir el público">
        <button
          type="button"
          role="radio"
          aria-checked={mode === 'perfil'}
          className={`${styles['mode']} ${mode === 'perfil' ? styles['mode-active'] : ''}`}
          onClick={() => switchMode('perfil')}
        >
          <Layers size={18} />
          <span>
            <strong>Por perfil</strong>
            <small>Empresas, profesionales, managers… Incluye a quien entre después.</small>
          </span>
        </button>
        <button
          type="button"
          role="radio"
          aria-checked={mode === 'personas'}
          className={`${styles['mode']} ${mode === 'personas' ? styles['mode-active'] : ''}`}
          onClick={() => switchMode('personas')}
        >
          <UserCheck size={18} />
          <span>
            <strong>Personas concretas</strong>
            <small>Elige a quién se la envías, una a una o por grupo.</small>
          </span>
        </button>
      </div>

      {mode === 'perfil' && (
        <>
          <div className={styles['roles-head']}>
            <span>Perfiles</span>
            <button
              type="button"
              className={styles['link']}
              onClick={() => set({ ...value, roles: everyone ? [] : [...DEFAULT_ROLES] })}
            >
              {everyone ? 'Quitar todos' : 'Empresas y todos los profesionales'}
            </button>
          </div>
          <div className={styles['roles']}>
            {ROLES.map(({ value: role, label, hint, icon: Icon }) => {
              const active = roles.includes(role)
              return (
                <button
                  key={role}
                  type="button"
                  aria-pressed={active}
                  className={`${styles['role']} ${active ? styles['role-active'] : ''}`}
                  onClick={() => set({ ...value, roles: allRoles.filter(r => r === role ? !active : roles.includes(r)) })}
                >
                  <span className={styles['role-icon']}><Icon size={15} /></span>
                  <span className={styles['role-text']}>
                    <strong>{label}</strong>
                    <small>{hint}</small>
                  </span>
                  {roleCounts[role] !== undefined && <span className={styles['role-count']}>{roleCounts[role]}</span>}
                </button>
              )
            })}
          </div>

          <label className={styles['switch']}>
            <input
              type="checkbox"
              checked={expanded}
              onChange={(e) => toggleExpanded(e.target.checked)}
            />
            <span>
              Acotar por empresa, país o grupo
              <small>Por ejemplo, solo los profesionales de una o dos empresas.</small>
            </span>
          </label>

          {expanded && (
            <div className={styles['body']}>
              <div className={styles['tabs']}>
                {TABS.map(({ value: tabValue, label, icon: Icon }) => (
                  <button
                    key={tabValue}
                    type="button"
                    className={`${styles['tab']} ${tab === tabValue ? styles['active'] : ''}`}
                    onClick={() => { setTab(tabValue); setSearch('') }}
                  >
                    <Icon size={14} /> {label}
                  </button>
                ))}
              </div>

              <input
                type="search"
                className={styles['search']}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={tab === 'paises' ? 'Buscar país...' : `Buscar ${tab}...`}
              />

              <div className={styles['options']}>
                {filtered.length === 0 ? (
                  <p className={styles['empty']}>
                    {tab === 'grupos'
                      ? 'No hay grupos de audiencia creados todavía. Se gestionan desde Tools › Correos.'
                      : 'Sin resultados.'}
                  </p>
                ) : tab === 'paises' ? (
                  (filtered as string[]).map(country => (
                    <label key={country} className={styles['option']}>
                      <input
                        type="checkbox"
                        checked={value.countries.includes(country)}
                        onChange={() => set({ ...value, countries: toggle(value.countries, country) })}
                      />
                      <span>{country}</span>
                    </label>
                  ))
                ) : (
                  (filtered as { id: number; name: string; count: number }[]).map(option => {
                    const selected = tab === 'empresas'
                      ? value.company_ids.includes(option.id)
                      : value.group_ids.includes(option.id)
                    return (
                      <label key={option.id} className={styles['option']}>
                        <input
                          type="checkbox"
                          checked={selected}
                          onChange={() => set(tab === 'empresas'
                            ? { ...value, company_ids: toggle(value.company_ids, option.id) }
                            : { ...value, group_ids: toggle(value.group_ids, option.id) })}
                        />
                        <span>{option.name}</span>
                        <small>{option.count}</small>
                      </label>
                    )
                  })
                )}
              </div>

              {chips.length > 0 && (
                <div className={styles['chips']}>
                  {chips.map(chip => (
                    <span key={chip.key} className={styles['chip']}>
                      {chip.label}
                      <button type="button" onClick={chip.remove} aria-label={`Quitar ${chip.label}`}>
                        <X size={12} />
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>
          )}
        </>
      )}

      {mode === 'personas' && (
        <RecipientSelector
          value={{ userIds: value.user_ids ?? [], groupIds: value.group_ids, expressContacts: [] }}
          onChange={(v) => set({ ...value, user_ids: v.userIds, group_ids: v.groupIds ?? [] })}
          roleFilters={PEOPLE_ROLE_FILTERS}
          eligible={isEligible}
        />
      )}

      {/* El alcance se muestra siempre: es la respuesta a "¿a cuánta gente le
          va a llegar esto?". Alcance cero es el error caro de esta pantalla,
          por eso el bloque cambia de tono. */}
      <div className={`${styles['reach']} ${preview?.reach === 0 && !isPreviewing ? styles['reach-empty'] : ''}`}>
        <strong>{isPreviewing ? '…' : preview?.reach ?? 0}</strong>
        <span>
          {preview?.reach === 0 && !isPreviewing
            ? mode === 'personas' ? 'Nadie recibirá esta novedad. Elige al menos una persona o un grupo.' : 'Nadie recibirá esta novedad. Revisa los perfiles y filtros.'
            : preview?.reach === 1 ? 'persona recibirá esta novedad' : 'personas recibirán esta novedad'}
          {preview && preview.by_audience.length > 0 && (
            <small>
              {preview.by_audience
                .map(row => `${row.reach} ${STAT_LABELS[row.user_type] ?? row.user_type}`)
                .join(' · ')}
            </small>
          )}
        </span>
      </div>
    </div>
  )
}
