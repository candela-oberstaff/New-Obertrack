import { useEffect, useMemo, useState } from 'react'
import { ArrowLeft, Save, ArrowUp, ArrowDown, Trash2, Plus } from 'lucide-react'

import { Select } from '../ui'
import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionBlock, type InductionProgram } from '../../services/induction.service'
import {
  certificateService,
  certificateDownloadUrl,
  templateImageUrl,
  type CertificateTemplate,
  type ProgramCertificates,
} from '../../services/certificate.service'
import { FileCheck, Download } from 'lucide-react'
import type { TutorialAudienceOption } from '../../types/tutorials'
import { BadgePicker, buildBadgePresets, type BadgeDraft } from '../Badges/BadgePicker'
import { DEFAULT_PROGRAM_BADGE } from '../Badges/badgeCatalog'
import styles from './InductionSettings.module.css'

interface Props {
  /** null = programa nuevo. */
  programId: number | null
  /** Biblioteca completa, para agregar bloques a la secuencia. */
  library: InductionBlock[]
  /** Empresas elegibles para asignar. */
  companies: TutorialAudienceOption[]
  /** Programas existentes, para copiar una insignia ya definida. */
  allPrograms?: InductionProgram[]
  onSaved: (program: InductionProgram) => void
  onBack: () => void
}

/**
 * Editor de un programa: reglas del portero, secuencia ordenada de bloques y
 * empresas asignadas. Todo se guarda de una vez: el programa, luego su
 * secuencia y luego sus empresas.
 */
export default function InductionProgramEditor({ programId, library, companies, allPrograms = [], onSaved, onBack }: Props) {
  const { success, error: showError } = useNotification()

  const [loading, setLoading] = useState(programId !== null)
  const [isDefault, setIsDefault] = useState(false)
  const [wasDefault, setWasDefault] = useState(false)
  const [isActive, setIsActive] = useState(true)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [defaultPassing, setDefaultPassing] = useState(70)
  const [maxAttempts, setMaxAttempts] = useState(3)
  const [blockIds, setBlockIds] = useState<number[]>([])
  const [companyIds, setCompanyIds] = useState<number[]>([])
  const [addBlockId, setAddBlockId] = useState<number>(0)
  const [companySearch, setCompanySearch] = useState('')
  const [badge, setBadge] = useState<BadgeDraft>({ title: '', ...DEFAULT_PROGRAM_BADGE })
  const [templateId, setTemplateId] = useState<number>(0)
  const [templates, setTemplates] = useState<CertificateTemplate[]>([])
  const [issued, setIssued] = useState<ProgramCertificates | null>(null)

  useEffect(() => {
    if (programId === null) return
    certificateService
      .forProgram(programId)
      .then(setIssued)
      .catch(() => setIssued(null))
  }, [programId])

  useEffect(() => {
    certificateService
      .listTemplates()
      .then(setTemplates)
      .catch(() => setTemplates([]))
  }, [])
  const presets = buildBadgePresets(library, allPrograms, programId !== null ? { kind: 'program', id: programId } : undefined)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (programId === null) return
    let cancelled = false
    inductionService
      .getProgram(programId)
      .then((p) => {
        if (cancelled) return
        setName(p.name)
        setDescription(p.description)
        setDefaultPassing(p.default_passing_score)
        setMaxAttempts(p.max_attempts)
        setIsDefault(p.is_default)
        setWasDefault(p.is_default)
        setIsActive(p.is_active)
        setBlockIds(p.blocks.map((b) => b.id))
        setCompanyIds(p.company_ids)
        setBadge({
          title: p.badge_title || '',
          icon: p.badge_icon || DEFAULT_PROGRAM_BADGE.icon,
          color: p.badge_color || DEFAULT_PROGRAM_BADGE.color,
        })
        setTemplateId(p.certificate_template_id ?? 0)
      })
      .catch(() => showError('No se pudo cargar el programa.'))
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [programId, showError])

  const blockById = useMemo(() => new Map(library.map((b) => [b.id, b])), [library])
  const available = library.filter((b) => !blockIds.includes(b.id))

  const filteredCompanies = useMemo(() => {
    const q = companySearch.trim().toLowerCase()
    if (!q) return companies
    return companies.filter((c) => c.name.toLowerCase().includes(q))
  }, [companies, companySearch])

  const move = (index: number, dir: -1 | 1) => {
    const next = [...blockIds]
    const target = index + dir
    if (target < 0 || target >= next.length) return
    ;[next[index], next[target]] = [next[target], next[index]]
    setBlockIds(next)
  }

  const toggleCompany = (id: number) =>
    setCompanyIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))

  const addBlock = () => {
    if (!addBlockId) return
    setBlockIds([...blockIds, addBlockId])
    setAddBlockId(0)
  }

  const handleSave = async () => {
    if (!name.trim()) {
      showError('El programa necesita un nombre.')
      return
    }
    setSaving(true)
    try {
      const input = {
        name: name.trim(),
        description: description.trim(),
        default_passing_score: defaultPassing,
        max_attempts: maxAttempts,
        is_default: isDefault,
        is_active: isActive,
        badge_title: badge.title.trim(),
        badge_icon: badge.icon,
        badge_color: badge.color,
        certificate_template_id: templateId,
      }
      const base =
        programId === null
          ? await inductionService.createProgram(input)
          : await inductionService.updateProgram(programId, input)
      await inductionService.setProgramBlocks(base.id, blockIds)
      const saved = await inductionService.setProgramCompanies(base.id, companyIds)
      success(programId === null ? 'Programa creado.' : 'Programa guardado.')
      onSaved(saved)
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar el programa.')
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <p className={styles.muted}>Cargando programa...</p>

  const pendingSummary =
    blockIds.length === 0
      ? 'Sin bloques: el programa no se podrá emitir.'
      : `${blockIds.length} ${blockIds.length === 1 ? 'bloque' : 'bloques'} · ${companyIds.length} ${
          companyIds.length === 1 ? 'empresa' : 'empresas'
        }`

  return (
    <div>
      <div className={styles.editorHead}>
        <button type="button" className={styles.backBtn} onClick={onBack}>
          <ArrowLeft size={14} /> Programas
        </button>
        <h3 className={styles.keyTitle}>{programId === null ? 'Nuevo programa' : name || 'Programa'}</h3>
        {wasDefault && <span className={styles.tagPrimary}>Por defecto</span>}
      </div>

      {/* --- 1. Datos --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>1</span>
          <h3 className={styles.keyTitle}>Datos y reglas</h3>
        </div>
        <p className={styles.sectionIntro}>
          El mínimo por defecto aplica a los bloques que no traen uno propio. Los intentos se
          cuentan por bloque: agotarlos en cualquiera bloquea al profesional.
        </p>

        <div className={styles.grid}>
          <div className={`${styles.field} ${styles.fieldWide}`}>
            <label htmlFor="program-name">Nombre del programa</label>
            <input
              id="program-name"
              type="text"
              placeholder="Ej. Inducción general"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus={programId === null}
            />
          </div>

          <div className={`${styles.field} ${styles.fieldWide}`}>
            <label htmlFor="program-description">Descripción (opcional)</label>
            <textarea
              id="program-description"
              placeholder="Para quién es y qué cubre."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="program-passing">Mínimo aprobatorio por defecto (%)</label>
            <input
              id="program-passing"
              type="number"
              min={0}
              max={100}
              value={defaultPassing}
              onChange={(e) => setDefaultPassing(Number(e.target.value))}
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="program-attempts">Intentos permitidos por bloque</label>
            <input
              id="program-attempts"
              type="number"
              min={1}
              max={10}
              value={maxAttempts}
              onChange={(e) => setMaxAttempts(Number(e.target.value))}
            />
          </div>
        </div>

        <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap', marginTop: 18 }}>
          <label className={styles.checkRow}>
            <input
              type="checkbox"
              checked={isDefault}
              // El por defecto no se desmarca desde aquí: se marca otro y este
              // deja de serlo solo.
              disabled={wasDefault}
              onChange={(e) => setIsDefault(e.target.checked)}
            />
            Programa por defecto (lo reciben las empresas sin asignación)
          </label>
          <label className={styles.checkRow}>
            <input
              type="checkbox"
              checked={isActive}
              disabled={wasDefault}
              onChange={(e) => setIsActive(e.target.checked)}
            />
            Activo
          </label>
        </div>
        {wasDefault && (
          <p className={styles.hint}>
            El programa por defecto no se puede apagar ni desmarcar. Para cambiarlo, marca otro
            programa como por defecto (desde la lista, con la estrella).
          </p>
        )}
      </div>

      {/* --- 2. Secuencia --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>2</span>
          <h3 className={styles.keyTitle}>Secuencia de bloques</h3>
          <span className={blockIds.length > 0 ? styles.tagOk : styles.tagWarn}>
            {blockIds.length} {blockIds.length === 1 ? 'bloque' : 'bloques'}
          </span>
        </div>
        <p className={styles.sectionIntro}>
          El profesional los recorre en este orden. No se abre un bloque sin aprobar el anterior.
        </p>

        {blockIds.length === 0 ? (
          <div className={styles.empty}>
            {library.length === 0
              ? 'La biblioteca está vacía: crea un bloque en la pestaña Bloques y vuelve aquí.'
              : 'Sin bloques. Agrega al menos uno para que el programa se pueda emitir.'}
          </div>
        ) : (
          <div className={styles.seqList}>
            {blockIds.map((id, index) => {
              const b = blockById.get(id)
              const passing = b?.passing_score ?? defaultPassing
              return (
                <div key={id} className={styles.seqItem}>
                  <span className={styles.seqNum}>{index + 1}</span>
                  <div className={styles.seqMain}>
                    <span className={styles.seqTitle}>{b?.name ?? `Bloque ${id}`}</span>
                    <span className={styles.seqMeta}>
                      {b?.tutorial_title || 'Sin video'} · {b?.question_count ?? 0} preguntas · mínimo {passing}%
                    </span>
                  </div>
                  <div className={styles.rowActions}>
                    <button
                      type="button"
                      className={`${styles.iconBtn} ${styles.iconBtnNeutral}`}
                      title="Subir"
                      aria-label="Subir"
                      disabled={index === 0}
                      onClick={() => move(index, -1)}
                    >
                      <ArrowUp size={15} />
                    </button>
                    <button
                      type="button"
                      className={`${styles.iconBtn} ${styles.iconBtnNeutral}`}
                      title="Bajar"
                      aria-label="Bajar"
                      disabled={index === blockIds.length - 1}
                      onClick={() => move(index, 1)}
                    >
                      <ArrowDown size={15} />
                    </button>
                    <button
                      type="button"
                      className={styles.iconBtn}
                      title="Quitar del programa"
                      aria-label="Quitar del programa"
                      onClick={() => setBlockIds(blockIds.filter((x) => x !== id))}
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        )}

        {available.length > 0 && (
          <div className={styles.seqAdd}>
            <div className={styles.field} style={{ flex: 1, minWidth: 240 }}>
              <label>Agregar bloque de la biblioteca</label>
              <Select
                fullWidth
                value={addBlockId}
                onChange={(v) => setAddBlockId(Number(v) || 0)}
                options={[
                  { value: 0, label: '— Elegir —' },
                  ...available.map((b) => ({
                    value: b.id,
                    label: `${b.name} · ${b.question_count} preguntas`,
                  })),
                ]}
              />
            </div>
            <button type="button" className={styles.ghostBtn} disabled={!addBlockId} onClick={addBlock}>
              <Plus size={16} /> Agregar
            </button>
          </div>
        )}
      </div>

      {/* --- 3. Empresas --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>3</span>
          <h3 className={styles.keyTitle}>Empresas asignadas</h3>
          <span className={styles.tag}>
            {companyIds.length} {companyIds.length === 1 ? 'empresa' : 'empresas'}
          </span>
        </div>
        <p className={styles.sectionIntro}>
          Quien se contrate en estas empresas recibe este programa. Una empresa solo puede estar
          en un programa: si ya estaba en otro, pasa a este.
          {isDefault && ' Este es el programa por defecto, así que además lo reciben todas las empresas sin asignación.'}
        </p>
        <input
          type="text"
          className={styles.searchInput}
          placeholder="Buscar empresa..."
          value={companySearch}
          onChange={(e) => setCompanySearch(e.target.value)}
        />
        {companies.length === 0 ? (
          <div className={styles.empty}>No hay empresas activas.</div>
        ) : filteredCompanies.length === 0 ? (
          <div className={styles.empty}>Ninguna empresa coincide con "{companySearch}".</div>
        ) : (
          <div className={styles.checkList}>
            {filteredCompanies.map((c) => (
              <label key={c.id} className={styles.checkItem}>
                <input type="checkbox" checked={companyIds.includes(c.id)} onChange={() => toggleCompany(c.id)} />
                <span>{c.name}</span>
                <span className={styles.count} title="Profesionales activos">
                  {c.count}
                </span>
              </label>
            ))}
          </div>
        )}
      </div>

      {/* --- 4. Insignia --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>4</span>
          <h3 className={styles.keyTitle}>Insignia del programa</h3>
        </div>
        <p className={styles.sectionIntro}>
          Es la medalla grande: se gana al completar todos los bloques. Además, quien lo haga sin
          fallar ningún intento gana "A la primera", y quien saque 100% en todo gana "Impecable".
        </p>
        <BadgePicker value={badge} fallbackTitle={name} presets={presets} onChange={setBadge} />
      </div>

      {/* --- 5. Certificado --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>5</span>
          <h3 className={styles.keyTitle}>Certificado</h3>
          {templateId > 0 && <span className={styles.tagOk}>Certifica</span>}
        </div>
        <p className={styles.sectionIntro}>
          Al completar el programa se emite un PDF con el diseño elegido, el nombre, la fecha y un
          código de verificación. Se descarga desde la plataforma; no se envía por correo.
          {templates.length === 0 && ' Todavía no hay plantillas: créalas en la pestaña Certificados.'}
        </p>
        <div style={{ display: 'flex', gap: 20, alignItems: 'flex-start', flexWrap: 'wrap' }}>
          <div className={styles.field} style={{ width: 420, maxWidth: '100%' }}>
            <label>Plantilla de certificado</label>
            <Select
              fullWidth
              value={templateId}
              onChange={(v) => setTemplateId(Number(v) || 0)}
              options={[
                { value: 0, label: 'Sin certificado' },
                ...templates.map((t) => ({
                  value: t.id,
                  label: `${t.name} · ${t.orientation === 'L' ? 'horizontal' : 'vertical'}`,
                })),
              ]}
            />
          </div>
          {(() => {
            const chosen = templates.find((t) => t.id === templateId)
            if (!chosen) return null
            const landscape = chosen.orientation === 'L'
            return (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6, alignItems: 'center' }}>
                <div
                  style={{
                    width: landscape ? 180 : 128,
                    height: landscape ? 128 : 180,
                    borderRadius: 10,
                    overflow: 'hidden',
                    border: '1px solid #e2e8f0',
                    background: '#f8fafc',
                    boxShadow: '0 4px 10px rgba(6, 11, 35, 0.08)',
                  }}
                >
                  <img
                    src={templateImageUrl(chosen.image_filename)}
                    alt={`Diseño ${chosen.name}`}
                    style={{ width: '100%', height: '100%', objectFit: 'fill' }}
                  />
                </div>
                <span className={styles.hint} style={{ margin: 0 }}>Así se ve el diseño</span>
              </div>
            )
          })()}
        </div>

        {issued && programId !== null && (
          <div style={{ marginTop: 20 }}>
            <div className={styles.sectionHead} style={{ marginBottom: 8 }}>
              <h3 className={styles.keyTitle} style={{ fontSize: 14 }}>
                Certificados emitidos en este programa
              </h3>
              <span className={issued.total > 0 ? styles.tagOk : styles.tag}>{issued.total}</span>
            </div>
            {issued.total === 0 ? (
              <p className={styles.muted}>Todavía no se ha emitido ninguno. Se emiten al completar el programa.</p>
            ) : (
              <div className={styles.list} style={{ maxWidth: 720 }}>
                {issued.recent.map((c) => (
                  <div key={c.id} className={styles.seqItem}>
                    <FileCheck size={16} color="#15803d" />
                    <div className={styles.seqMain}>
                      <span className={styles.seqTitle}>{c.user_name || `Usuario ${c.user_id}`}</span>
                      <span className={styles.seqMeta}>
                        {c.code} · {new Date(c.issued_at).toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })}
                        {c.reissued_at ? ' · reemitido' : ''}
                      </span>
                    </div>
                    <a
                      href={certificateDownloadUrl(c.code)}
                      download
                      className={`${styles.iconBtn} ${styles.iconBtnNeutral}`}
                      title="Descargar"
                      aria-label={`Descargar certificado de ${c.user_name}`}
                    >
                      <Download size={15} />
                    </a>
                  </div>
                ))}
                {issued.total > issued.recent.length && (
                  <p className={styles.hint}>Se muestran los {issued.recent.length} más recientes de {issued.total}.</p>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      <div className={styles.stickyBar}>
        <span className={styles.muted}>{pendingSummary}</span>
        <button type="button" className={styles.ghostBtn} onClick={onBack} disabled={saving}>
          Cancelar
        </button>
        <button type="button" className={styles.saveBtn} disabled={saving} onClick={handleSave}>
          <Save size={16} /> {saving ? 'Guardando...' : programId === null ? 'Crear programa' : 'Guardar programa'}
        </button>
      </div>
    </div>
  )
}
