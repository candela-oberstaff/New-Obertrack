import { useEffect, useMemo, useState } from 'react'
import {
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  ArrowDown,
  Award,
  Building2,
  Check,
  Download,
  FileCheck,
  Layers,
  Plus,
  Save,
  Search,
  Send,
  Star,
  Target,
  Trash2,
} from 'lucide-react'

import { Select } from '../ui'
import { Modal } from '../ui/Modal'
import { useDirtySnapshot } from '../ui/useCloseGuard'
import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionBlock, type InductionProgram } from '../../services/induction.service'
import { emailService } from '../../services/emailService'
import { buildCompanyIndex, companyNameOf, type CompanyAwareUser } from '../../lib/recipientCompany'
import {
  certificateService,
  certificateDownloadUrl,
  templateImageUrl,
  type CertificateTemplate,
  type ProgramCertificates,
} from '../../services/certificate.service'
import type { TutorialAudienceOption } from '../../types/tutorials'
import { BadgePicker, buildBadgePresets, type BadgeDraft } from '../Badges/BadgePicker'
import { BadgeMedallion } from '../Badges/BadgeMedallion'
import { DEFAULT_PROGRAM_BADGE } from '../Badges/badgeCatalog'
import ReadinessChecklist from './ReadinessChecklist'
import SendProgramModal from './SendProgramModal'
import { LOW_PASSING_SCORE, isReady, programIssues } from './inductionReadiness'
import styles from './InductionSettings.module.css'

const STEPS = ['Datos', 'Bloques', 'Destinatarios', 'Revisar']

/** Iniciales para el avatar de una opción: «Marta Solís» → «MS». */
function initials(name: string): string {
  // Solo cuenta lo que empieza con letra: «Oberstaff (prueba)» → «OP», no «O(».
  const parts = name
    .trim()
    .split(/\s+/)
    .map((w) => w.replace(/^[^\p{L}]+/u, ''))
    .filter(Boolean)
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase() || '·'
}

interface PickOption {
  id: number
  name: string
  sub: string
  /** Texto en el que busca el buscador. */
  haystack: string
}

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
  onClose: () => void
  /** Lleva a la biblioteca de bloques cuando está vacía. */
  onGoToBlocks?: () => void
}

/**
 * Modal con el asistente de un programa, en cuatro pasos: datos, secuencia de
 * bloques, empresas que lo reciben y revisión (reglas, estado, certificado e
 * insignia). Todo se guarda de una vez: el programa, luego su secuencia y
 * luego sus empresas.
 */
export default function InductionProgramModal({
  programId,
  library,
  companies,
  allPrograms = [],
  onSaved,
  onClose,
  onGoToBlocks,
}: Props) {
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
  const [userIds, setUserIds] = useState<number[]>([])
  const [pros, setPros] = useState<PickOption[]>([])
  const [prosLoading, setProsLoading] = useState(true)
  const [proSearch, setProSearch] = useState('')
  const [badge, setBadge] = useState<BadgeDraft>({ title: '', ...DEFAULT_PROGRAM_BADGE })
  const [templateId, setTemplateId] = useState<number>(0)
  const [templates, setTemplates] = useState<CertificateTemplate[]>([])
  const [issued, setIssued] = useState<ProgramCertificates | null>(null)
  const [saving, setSaving] = useState(false)
  // Programa recién guardado que se está enviando (abre la ventana de envío).
  const [sendingProgram, setSendingProgram] = useState<InductionProgram | null>(null)
  // Uno nuevo empieza por el nombre; uno existente, por el resumen, desde
  // donde se salta a lo que haya que cambiar.
  const [step, setStep] = useState(programId === null ? 0 : 3)
  // Cerrar con Escape, la X o fuera del modal pide confirmar si hay cambios.
  // En un programa existente la foto se toma cuando terminan de cargar sus datos.
  const isDirty = useDirtySnapshot(
    { name, description, defaultPassing, maxAttempts, isDefault, isActive, blockIds, companyIds, userIds, badge, templateId },
    !loading
  )

  useEffect(() => {
    if (programId === null) return
    certificateService
      .forProgram(programId)
      .then(setIssued)
      .catch(() => setIssued(null))
  }, [programId])

  // Profesionales elegibles, con su empresa (que no viene en su propia fila).
  useEffect(() => {
    emailService
      .getAvailableRecipients()
      .then((resp: any) => {
        const all: CompanyAwareUser[] = Array.isArray(resp) ? resp : (resp?.data ?? resp?.users ?? [])
        const index = buildCompanyIndex(all)
        setPros(
          all
            .filter((u) => u.user_type === 'profesional')
            .map((u) => {
              const company = companyNameOf(u, index)
              const name = (u.name || u.email || `Usuario ${u.id}`).trim()
              return {
                id: u.id,
                name,
                sub: company || u.email || 'Sin empresa',
                haystack: `${name} ${u.email ?? ''} ${company}`.toLowerCase(),
              }
            })
            .sort((a, b) => a.name.localeCompare(b.name, 'es'))
        )
      })
      .catch(() => setPros([]))
      .finally(() => setProsLoading(false))
  }, [])

  useEffect(() => {
    certificateService
      .listTemplates()
      .then(setTemplates)
      .catch(() => setTemplates([]))
  }, [])
  const presets = buildBadgePresets(allPrograms, programId)

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
        setUserIds(p.user_ids ?? [])
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

  // Empresas asignadas de antes (ya no se eligen aquí), con su nombre.
  const companyNames = useMemo(
    () => companyIds.map((id) => companies.find((c) => c.id === id)?.name).filter((n): n is string => !!n),
    [companyIds, companies]
  )

  const filteredPros = useMemo(() => {
    const q = proSearch.trim().toLowerCase()
    return q ? pros.filter((p) => p.haystack.includes(q)) : pros
  }, [pros, proSearch])

  const move = (index: number, dir: -1 | 1) => {
    const next = [...blockIds]
    const target = index + dir
    if (target < 0 || target >= next.length) return
    ;[next[index], next[target]] = [next[target], next[index]]
    setBlockIds(next)
  }

  // thenSend: «Guardar y enviar» guarda primero y luego abre el envío masivo
  // con el programa ya guardado (los destinatarios salen de lo guardado).
  const handleSave = async (thenSend = false) => {
    if (!name.trim()) {
      showError('El programa necesita un nombre.')
      setStep(0)
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
      await inductionService.setProgramCompanies(base.id, companyIds)
      const saved = await inductionService.setProgramUsers(base.id, userIds)
      success(programId === null ? 'Programa creado.' : 'Programa guardado.')
      onSaved(saved)
      if (thenSend) {
        setSendingProgram(saved)
      } else if (programId === null) {
        // Al crear, de vuelta a la lista: el programa ya aparece ahí.
        onClose()
      }
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar el programa.')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <Modal isOpen onClose={onClose} size="xl" title="Programa" ariaLabel="Cargando programa">
        <p className={styles.muted}>Cargando programa...</p>
      </Modal>
    )
  }

  const sequence = blockIds.map((id) => blockById.get(id)).filter((b): b is InductionBlock => !!b)
  // Lo que falta, calculado sobre el borrador: cambia mientras se edita.
  const issues = programIssues({
    isActive,
    isDefault,
    defaultPassingScore: defaultPassing,
    blocks: sequence,
    recipientCount: companyIds.length + userIds.length,
    hasCertificate: templateId > 0,
  })
  const chosenTemplate = templates.find((t) => t.id === templateId) ?? null
  const blocksReady = sequence.length > 0 && sequence.every((b) => b.question_count > 0)
  const audienceParts = [
    companyIds.length > 0 && `${companyIds.length} ${companyIds.length === 1 ? 'empresa' : 'empresas'}`,
    userIds.length > 0 && `${userIds.length} ${userIds.length === 1 ? 'profesional' : 'profesionales'}`,
  ].filter(Boolean)
  const audience = isDefault
    ? audienceParts.length > 0
      ? `Por defecto + ${audienceParts.join(' y ')}`
      : 'Todas las empresas sin asignación'
    : audienceParts.length > 0
      ? audienceParts.join(' y ')
      : 'Nadie todavía'

  // --- Asistente por pasos ---------------------------------------------------
  const goTo = (target: number) => {
    if (target > 0 && !name.trim()) {
      showError('Primero ponle un nombre al programa.')
      setStep(0)
      return
    }
    setStep(Math.max(0, Math.min(STEPS.length - 1, target)))
  }
  const isLast = step === STEPS.length - 1
  const stepDone = [
    name.trim() !== '',
    blocksReady,
    isDefault || companyIds.length + userIds.length > 0,
    isReady(issues),
  ]
  const stepSub = [
    name.trim() || 'Nombre y descripción',
    `${blockIds.length} ${blockIds.length === 1 ? 'bloque' : 'bloques'}`,
    audience,
    isReady(issues) ? 'Todo listo' : 'Con avisos',
  ]
  const badgeTitle = badge.title.trim() || name.trim() || 'Insignia del programa'

  const footer = (
    <>
      <span className={`${styles.muted} ${styles.modalFooterNote}`}>
        Paso {step + 1} de {STEPS.length}
      </span>
      {step === 0 ? (
        <button type="button" className={styles.ghostBtn} onClick={onClose} disabled={saving}>
          {programId === null ? 'Cancelar' : 'Cerrar'}
        </button>
      ) : (
        <button type="button" className={styles.ghostBtn} onClick={() => goTo(step - 1)} disabled={saving}>
          <ArrowLeft size={16} /> Atrás
        </button>
      )}
      {/* Un programa ya creado se puede guardar desde cualquier paso. */}
      {programId !== null && !isLast && (
        <button type="button" className={styles.ghostBtn} disabled={saving} onClick={() => handleSave()}>
          <Save size={16} /> {saving ? 'Guardando...' : 'Guardar'}
        </button>
      )}
      {isLast && (
        <button
          type="button"
          className={styles.sendBtnLg}
          disabled={saving || !isActive || blockIds.length === 0}
          title={
            !isActive || blockIds.length === 0
              ? 'Para enviarlo, el programa tiene que estar activo y tener bloques'
              : 'Guarda el programa y elige a quién enviárselo ahora'
          }
          onClick={() => handleSave(true)}
        >
          <Send size={16} /> Guardar y enviar
        </button>
      )}
      {isLast ? (
        <button type="button" className={styles.saveBtn} disabled={saving} onClick={() => handleSave()}>
          <Save size={16} /> {saving ? 'Guardando...' : programId === null ? 'Crear programa' : 'Guardar programa'}
        </button>
      ) : (
        <button type="button" className={styles.saveBtn} onClick={() => goTo(step + 1)}>
          Siguiente <ArrowRight size={16} />
        </button>
      )}
    </>
  )

  const title = programId === null ? 'Nuevo programa' : name || 'Programa'

  return (
    <Modal
      isOpen
      onClose={onClose}
      size="xl"
      isDirty={isDirty}
      ariaLabel={title}
      title={
        <>
          {title}
          {wasDefault && <span className={`${styles.tagPrimary} ${styles.modalTitleTag}`}>Por defecto</span>}
        </>
      }
      footer={footer}
    >
      <ol className={`${styles.wizard} ${styles.modalWizard}`} aria-label="Pasos del programa">
        {STEPS.map((label, i) => (
          <li key={label}>
            <button
              type="button"
              className={i === step ? styles.wizardStepActive : styles.wizardStep}
              aria-current={i === step ? 'step' : undefined}
              onClick={() => goTo(i)}
            >
              <span className={styles.wizardNum} data-done={stepDone[i] && i !== step}>
                {stepDone[i] && i !== step ? <Check size={14} strokeWidth={3} /> : i + 1}
              </span>
              <span className={styles.stepText}>
                <span className={styles.stepLabel}>{label}</span>
                <span className={styles.stepSub}>{stepSub[i]}</span>
              </span>
            </button>
          </li>
        ))}
      </ol>

      {/* --- 1. Datos --- */}
      <div className={styles.section} hidden={step !== 0}>
        <h3 className={styles.wizardTitle}>¿Cómo se llama este programa?</h3>
        <p className={styles.wizardIntro}>
          Un programa es la lista ordenada de bloques que recorre el profesional al entrar. Puedes
          tener uno general y otros para empresas concretas.
        </p>
        <div className={styles.wizardSplit}>
          <div>
            <div className={styles.field}>
              <label htmlFor="program-name">Nombre del programa</label>
              <input
                id="program-name"
                type="text"
                placeholder="Ej. Inducción general"
                value={name}
                onChange={(e) => setName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') goTo(1)
                }}
                autoFocus={programId === null}
              />
            </div>
            <div className={styles.field} style={{ marginTop: 16 }}>
              <label htmlFor="program-description">Descripción (opcional)</label>
              <textarea
                id="program-description"
                placeholder="Para quién es y qué cubre."
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
          </div>

          {/* Vista previa en vivo, como la ve quien recorre la inducción. */}
          <aside className={styles.blockPreview} aria-label="Vista previa del programa">
            <span className={styles.blockPreviewLabel}>Así lo verá el profesional</span>
            <div className={styles.blockPreviewCard}>
              <span className={styles.blockPreviewKicker}>Tu inducción</span>
              <strong className={styles.blockPreviewName}>{name.trim() || 'Nombre del programa'}</strong>
              <p className={styles.blockPreviewDesc}>
                {description.trim() || 'Aquí aparece la descripción: para quién es y qué cubre.'}
              </p>
              <div className={styles.blockPreviewChips}>
                <span>
                  <Layers size={12} /> {blockIds.length} {blockIds.length === 1 ? 'bloque' : 'bloques'}
                </span>
                <span>
                  <Target size={12} /> Aprueba con {defaultPassing}%
                </span>
                <span>
                  <FileCheck size={12} /> {templateId > 0 ? 'Con certificado' : 'Sin certificado'}
                </span>
              </div>
              <div className={styles.blockPreviewBadge}>
                <BadgeMedallion icon={badge.icon} color={badge.color} size="sm" />
                <span>
                  Al completarlo gana <strong>{badgeTitle}</strong>
                </span>
              </div>
            </div>
          </aside>
        </div>
      </div>

      {/* --- 2. Bloques --- */}
      <div className={styles.section} hidden={step !== 1}>
        <h3 className={styles.wizardTitle}>Ordena los bloques</h3>
        <p className={styles.wizardIntro}>
          El profesional los recorre en este orden y no abre uno sin aprobar el anterior. Agrega
          bloques desde la biblioteca de la derecha.
        </p>
        <div className={styles.programSplit}>
          <div>
            <span className={styles.blockPreviewLabel}>Secuencia · {blockIds.length}</span>
            {blockIds.length === 0 ? (
              <div className={styles.empty} style={{ marginTop: 8 }}>
                Sin bloques todavía. Agrega el primero desde la biblioteca.
              </div>
            ) : (
              <div className={styles.seqList} style={{ marginTop: 8 }}>
                {blockIds.map((id, index) => {
                  const b = blockById.get(id)
                  const passing = b?.passing_score ?? defaultPassing
                  const noQuestions = (b?.question_count ?? 0) === 0
                  return (
                    <div key={id} className={styles.seqItem}>
                      <span className={styles.seqNum}>{index + 1}</span>
                      <div className={styles.seqMain}>
                        <span className={styles.seqTitle}>{b?.name ?? `Bloque ${id}`}</span>
                        <span className={styles.seqMeta}>
                          {b?.video_title || 'Sin video'} ·{' '}
                          <span style={noQuestions ? { color: '#b91c1c', fontWeight: 600 } : undefined}>
                            {b?.question_count ?? 0} preguntas
                          </span>{' '}
                          ·{' '}
                          <span style={passing < LOW_PASSING_SCORE ? { color: '#b45309', fontWeight: 600 } : undefined}>
                            mínimo {passing}%
                          </span>
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
          </div>

          <aside className={styles.libPanel} aria-label="Biblioteca de bloques">
            <span className={styles.blockPreviewLabel}>Biblioteca · {available.length} sin usar aquí</span>
            {library.length === 0 ? (
              <div className={styles.empty} style={{ marginTop: 8 }}>
                La biblioteca está vacía.{' '}
                {onGoToBlocks && (
                  <button type="button" className={styles.linkBtn} onClick={onGoToBlocks}>
                    Crear el primer bloque
                  </button>
                )}
              </div>
            ) : available.length === 0 ? (
              <p className={styles.hint}>Todos los bloques de la biblioteca ya están en este programa.</p>
            ) : (
              <div className={styles.libList}>
                {available.map((b) => (
                  <button
                    key={b.id}
                    type="button"
                    className={styles.libItem}
                    onClick={() => setBlockIds([...blockIds, b.id])}
                    title={`Agregar «${b.name}» al final`}
                  >
                    <span className={styles.seqMain}>
                      <span className={styles.seqTitle}>{b.name}</span>
                      <span className={styles.seqMeta}>
                        {b.video_title || 'Sin video'} · {b.question_count}{' '}
                        {b.question_count === 1 ? 'pregunta' : 'preguntas'}
                      </span>
                    </span>
                    <span className={styles.libAdd}>
                      <Plus size={14} /> Agregar
                    </span>
                  </button>
                ))}
              </div>
            )}
          </aside>
        </div>
      </div>

      {/* --- 3. Destinatarios --- */}
      <div className={styles.section} hidden={step !== 2}>
        <h3 className={styles.wizardTitle}>¿A quién se envía?</h3>
        <p className={styles.wizardIntro}>
          Elige a los profesionales que lo reciben. Les llegará cuando les toque la formación: al
          contratarlos o cuando Soporte los invite. Su programa manda sobre el de su empresa.
        </p>

        {/* Programas de antes podían asignarse a empresas enteras. Ya no se
            elige así, pero esas asignaciones siguen valiendo: se muestran para
            que no queden escondidas y se pueden quitar. */}
        {companyIds.length > 0 && (
          <div className={styles.readinessBox} style={{ marginTop: 0, marginBottom: 14 }}>
            <div className={styles.readinessItem} data-level="warning">
              <Building2 size={15} />
              <span>
                Este programa también está asignado a {companyIds.length}{' '}
                {companyIds.length === 1 ? 'empresa' : 'empresas'} de antes
                {companyNames.length > 0 && <> ({companyNames.join(', ')})</>}: lo recibe todo el que se
                contrate ahí.{' '}
                <button type="button" className={styles.linkBtn} onClick={() => setCompanyIds([])}>
                  Quitar esas empresas
                </button>
              </span>
            </div>
          </div>
        )}

        {(() => {
          const items = filteredPros
          const visibleIds = items.map((i) => i.id)
          const allVisibleSelected = visibleIds.length > 0 && visibleIds.every((id) => userIds.includes(id))
          const toggle = (id: number) =>
            setUserIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))
          return (
            <div className={styles.pickPanel}>
              <div className={styles.pickToolbar}>
                <div className={styles.videoSearch} style={{ margin: 0, flex: 1, maxWidth: 420 }}>
                  <Search size={15} />
                  <input
                    type="text"
                    placeholder="Buscar por nombre, correo o empresa..."
                    value={proSearch}
                    onChange={(e) => setProSearch(e.target.value)}
                    aria-label="Buscar profesional"
                  />
                </div>
                <span className={styles.muted} style={{ fontSize: 13 }}>
                  {userIds.length} {userIds.length === 1 ? 'profesional elegido' : 'profesionales elegidos'}
                </span>
                {items.length > 0 && (
                  <button
                    type="button"
                    className={styles.linkBtn}
                    onClick={() =>
                      setUserIds((prev) =>
                        allVisibleSelected
                          ? prev.filter((id) => !visibleIds.includes(id))
                          : [...prev, ...visibleIds.filter((id) => !prev.includes(id))]
                      )
                    }
                  >
                    {allVisibleSelected ? 'Quitar los visibles' : `Elegir los ${items.length} visibles`}
                  </button>
                )}
              </div>

              {prosLoading ? (
                <p className={styles.muted}>Cargando profesionales...</p>
              ) : pros.length === 0 ? (
                <div className={styles.empty}>No hay profesionales.</div>
              ) : items.length === 0 ? (
                <div className={styles.empty}>Nadie coincide con «{proSearch}».</div>
              ) : (
                <div className={styles.pickList}>
                  {items.map((item) => {
                    const on = userIds.includes(item.id)
                    return (
                      <button
                        key={item.id}
                        type="button"
                        role="checkbox"
                        aria-checked={on}
                        className={on ? styles.pickItemOn : styles.pickItem}
                        onClick={() => toggle(item.id)}
                      >
                        <span className={styles.pickAvatar}>{on ? <Check size={14} strokeWidth={3} /> : initials(item.name)}</span>
                        <span className={styles.pickText}>
                          <span className={styles.pickName}>{item.name}</span>
                          <span className={styles.pickSub}>{item.sub}</span>
                        </span>
                      </button>
                    )
                  })}
                </div>
              )}
            </div>
          )
        })()}

        {/* El por defecto cubre a todas las empresas sin asignación. */}
        <label className={styles.defaultRow} title={wasDefault ? 'Para cambiarlo, marca otro programa con la estrella desde la lista' : undefined}>
          <span className={styles.switch}>
            <input
              type="checkbox"
              checked={isDefault}
              // El por defecto no se desmarca desde aquí: se marca otro y este
              // deja de serlo solo.
              disabled={wasDefault}
              onChange={(e) => setIsDefault(e.target.checked)}
            />
            <span className={styles.slider} />
          </span>
          <span className={styles.audienceText}>
            <strong>
              <Star size={14} /> Programa por defecto
            </strong>
            <span>
              {wasDefault
                ? 'Lo reciben además todas las empresas sin programa. Para cambiarlo, marca otro con la estrella desde la lista.'
                : 'Además lo recibirán todas las empresas que no tengan un programa asignado. Solo puede haber uno.'}
            </span>
          </span>
        </label>
      </div>

      {/* --- 4. Revisar --- */}
      <div className={styles.section} hidden={step !== 3}>
        <h3 className={styles.wizardTitle}>Revisa y {programId === null ? 'crea' : 'guarda'} el programa</h3>
        <p className={styles.wizardIntro}>
          Las reglas, el estado y el certificado se ajustan aquí mismo.
        </p>

        {/* Una sola columna: primero la insignia, luego el resumen. La insignia es un
            único bloque: cómo la verá el profesional a la izquierda y su editor a la
            derecha (debajo en pantallas angostas). */}
        <div className={styles.reviewStack}>
          <section className={styles.badgePanel} aria-label="Insignia del programa">
            <div className={styles.badgePanelPreview}>
              <span className={styles.blockPreviewLabel}>Insignia que gana</span>
              <BadgeMedallion icon={badge.icon} color={badge.color} size="lg" />
              <strong className={styles.badgeCardTitle}>{badgeTitle}</strong>
              <span className={styles.badgeCardHint}>
                Se gana al completar todos los bloques. Además, «A la primera» si no falla ningún
                intento e «Impecable» si saca 100% en todo.
              </span>
            </div>
            <div className={styles.badgePanelEditor}>
              <BadgePicker value={badge} fallbackTitle={name} presets={presets} showPreview={false} onChange={setBadge} />
            </div>
          </section>

          <div>
            <div className={styles.reviewList}>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Nombre</span>
                <span className={styles.reviewValue}>{name || '—'}</span>
                <button type="button" className={styles.linkBtn} onClick={() => goTo(0)}>
                  Cambiar
                </button>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Bloques</span>
                <span className={styles.reviewValue}>
                  {blockIds.length} {blockIds.length === 1 ? 'bloque' : 'bloques'}
                  {sequence.length > 0 && <small>{sequence.map((b) => b.name).join(' → ')}</small>}
                </span>
                <button type="button" className={styles.linkBtn} onClick={() => goTo(1)}>
                  Cambiar
                </button>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Quién lo recibe</span>
                <span className={styles.reviewValue}>{audience}</span>
                <button type="button" className={styles.linkBtn} onClick={() => goTo(2)}>
                  Cambiar
                </button>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Mínimo para aprobar</span>
                <span className={styles.reviewValue}>
                  <span className={styles.percentInput}>
                    <input
                      id="program-passing"
                      type="number"
                      min={0}
                      max={100}
                      aria-label="Mínimo aprobatorio por defecto (%)"
                      value={defaultPassing}
                      onChange={(e) => setDefaultPassing(Number(e.target.value))}
                    />
                    %
                  </span>
                  <small>en los bloques que no traen uno propio</small>
                </span>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Intentos por bloque</span>
                <span className={styles.reviewValue}>
                  <span className={styles.percentInput}>
                    <input
                      id="program-attempts"
                      type="number"
                      min={1}
                      max={10}
                      aria-label="Intentos permitidos por bloque"
                      value={maxAttempts}
                      onChange={(e) => setMaxAttempts(Number(e.target.value))}
                    />
                  </span>
                  <small>agotarlos en cualquiera bloquea al profesional y abre una alerta en Soporte</small>
                </span>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Estado</span>
                <span className={styles.reviewValue}>
                  <span
                    className={styles.segmented}
                    role="radiogroup"
                    aria-label="Estado del programa"
                    title={wasDefault ? 'El programa por defecto no se puede apagar' : undefined}
                  >
                    <button
                      type="button"
                      role="radio"
                      aria-checked={isActive}
                      className={isActive ? styles.segmentActive : styles.segment}
                      onClick={() => setIsActive(true)}
                    >
                      Activo
                    </button>
                    <button
                      type="button"
                      role="radio"
                      aria-checked={!isActive}
                      className={!isActive ? styles.segmentActive : styles.segment}
                      disabled={wasDefault}
                      onClick={() => setIsActive(false)}
                    >
                      Apagado
                    </button>
                  </span>
                </span>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Certificado</span>
                <span className={styles.reviewValue} style={{ fontWeight: 500 }}>
                  <span style={{ minWidth: 260, flex: 1, maxWidth: 380 }}>
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
                  </span>
                  {templates.length === 0 && <small>Créalas en la pestaña Certificados.</small>}
                </span>
              </div>
            </div>

            <ReadinessChecklist
              issues={issues}
              readyText={
                isDefault
                  ? 'Listo: es el programa que reciben todas las empresas sin asignación.'
                  : 'Listo para recibir profesionales.'
              }
            />
          </div>

          {chosenTemplate && (
            <div className={styles.certCard}>
              <span className={styles.blockPreviewLabel}>Certificado</span>
              <img
                src={templateImageUrl(chosenTemplate.image_filename)}
                alt={`Diseño ${chosenTemplate.name}`}
                className={chosenTemplate.orientation === 'L' ? styles.certThumbL : styles.certThumbP}
              />
              <span className={styles.badgeCardHint}>
                Se emite al completar el programa, con el nombre, la fecha y un código de
                verificación. Se descarga desde la plataforma.
              </span>
            </div>
          )}
        </div>

        {issued && programId !== null && (
          <div style={{ marginTop: 24 }}>
            <div className={styles.sectionHead} style={{ marginBottom: 8 }}>
              <Award size={16} color="#64748b" />
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
                        {c.code} ·{' '}
                        {new Date(c.issued_at).toLocaleDateString('es-ES', {
                          day: '2-digit',
                          month: 'short',
                          year: 'numeric',
                        })}
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
                  <p className={styles.hint}>
                    Se muestran los {issued.recent.length} más recientes de {issued.total}.
                  </p>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Envío masivo tras «Guardar y enviar»: un modal dentro del asistente. */}
      {sendingProgram && (
        <SendProgramModal
          program={sendingProgram}
          onClose={() => {
            setSendingProgram(null)
            if (programId === null) onClose()
          }}
        />
      )}
    </Modal>
  )
}
