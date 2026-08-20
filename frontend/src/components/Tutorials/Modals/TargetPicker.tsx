import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Building2, Globe, Users, UserCog, X } from 'lucide-react'
import { tutorialService } from '../../../services/api'
import { isEmptyTarget } from '../../../types'
import type { TutorialAudience, TutorialTarget } from '../../../types'
import styles from './TargetPicker.module.css'

interface TargetPickerProps {
  audience: TutorialAudience
  value: TutorialTarget
  onChange: (target: TutorialTarget) => void
}

type Tab = 'empresas' | 'paises' | 'grupos'

const TABS: { value: Tab; label: string; icon: typeof Building2 }[] = [
  { value: 'empresas', label: 'Empresas', icon: Building2 },
  { value: 'paises', label: 'Países', icon: Globe },
  { value: 'grupos', label: 'Grupos', icon: Users },
]

/** Quita o agrega un elemento de una lista, sin repetirlo. */
function toggle<T>(list: T[], item: T): T[] {
  return list.includes(item) ? list.filter(i => i !== item) : [...list, item]
}

/**
 * Público objetivo de una novedad. Acota por encima del tipo de cuenta y los
 * criterios se combinan con Y: "profesionales de Acme que además estén en
 * Venezuela". Todo vacío = toda la audiencia.
 *
 * Lo importante no es el selector sino el contador: mientras se elige, se
 * consulta a cuánta gente llegaría. Acotar sin ver el alcance es disparar a
 * ciegas, que es justo lo que hacía falta arreglar.
 */
export function TargetPicker({ audience, value, onChange }: TargetPickerProps) {
  const [tab, setTab] = useState<Tab>('empresas')
  const [search, setSearch] = useState('')
  const [expanded, setExpanded] = useState(!isEmptyTarget(value))

  const { data: options } = useQuery({
    queryKey: ['tutorial-audience-options'],
    queryFn: () => tutorialService.getAudienceOptions(),
    staleTime: 5 * 60_000,
  })

  // Al plegar el bloque se limpia el público: dejarlo acotado sin que se vea
  // sería publicar creyendo que llega a todos.
  useEffect(() => {
    if (!expanded && !isEmptyTarget(value)) {
      onChange({ company_ids: [], countries: [], group_ids: [], managers_only: false })
    }
  }, [expanded, value, onChange])

  const { data: preview, isFetching: isPreviewing } = useQuery({
    queryKey: ['tutorial-audience-preview', audience, value],
    queryFn: () => tutorialService.previewAudience(audience, value),
    staleTime: 30_000,
  })

  const companies = options?.companies ?? []
  const countries = options?.countries ?? []
  const groups = options?.groups ?? []

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
      remove: () => onChange({ ...value, company_ids: value.company_ids.filter(i => i !== id) }),
    })),
    ...value.countries.map(country => ({
      key: `p${country}`,
      label: country,
      remove: () => onChange({ ...value, countries: value.countries.filter(c => c !== country) }),
    })),
    ...value.group_ids.map(id => ({
      key: `g${id}`,
      label: groups.find(g => g.id === id)?.name ?? `Grupo ${id}`,
      remove: () => onChange({ ...value, group_ids: value.group_ids.filter(i => i !== id) }),
    })),
  ]

  return (
    <div className={styles['picker']}>
      <label className={styles['switch']}>
        <input
          type="checkbox"
          checked={expanded}
          onChange={(e) => setExpanded(e.target.checked)}
        />
        <span>
          Acotar el público
          <small>Por empresa, país, grupo o solo quienes tienen equipo a cargo.</small>
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
                    onChange={() => onChange({ ...value, countries: toggle(value.countries, country) })}
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
                      onChange={() => onChange(tab === 'empresas'
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

          {/* Con la audiencia puesta en Managers este filtro ya está aplicado:
              mostrarlo sería ofrecer dos veces la misma decisión. */}
          {audience !== 'manager' && (
          <label className={styles['managers']}>
            <input
              type="checkbox"
              checked={value.managers_only}
              onChange={(e) => onChange({ ...value, managers_only: e.target.checked })}
            />
            <UserCog size={15} />
            <span>
              Solo quienes tienen equipo a cargo
              {/* La marca de equipo a cargo la llevan las personas, no las
                  cuentas de empresa: sin decirlo, un alcance sin empresas
                  parece un error del contador. */}
              <small>Managers y supervisores. Las cuentas de empresa no llevan esta marca.</small>
            </span>
          </label>
          )}

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

      {/* El alcance se muestra siempre, acotado o no: es la respuesta a
          "¿a cuánta gente le va a llegar esto?". */}
      {/* Alcance cero es el error caro de esta pantalla: se publica creyendo
          que llega a alguien. Por eso el bloque cambia de tono. */}
      <div className={`${styles['reach']} ${preview?.reach === 0 && !isPreviewing ? styles['reach-empty'] : ''}`}>
        <strong>{isPreviewing ? '…' : preview?.reach ?? 0}</strong>
        <span>
          {preview?.reach === 0 && !isPreviewing
            ? 'Nadie recibirá esta novedad. Revisa los criterios.'
            : preview?.reach === 1 ? 'persona recibirá esta novedad' : 'personas recibirán esta novedad'}
          {preview && preview.by_audience.length > 0 && (
            <small>
              {preview.by_audience
                .map(row => `${row.reach} ${row.user_type === 'empleador' ? 'empresas' : 'profesionales'}`)
                .join(' · ')}
            </small>
          )}
        </span>
      </div>
    </div>
  )
}
