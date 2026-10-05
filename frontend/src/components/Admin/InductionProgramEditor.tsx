import { useEffect, useMemo, useState } from 'react'
import {
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  ArrowDown,
  Award,
  Check,
  Download,
  FileCheck,
  Layers,
  Palette,
  Plus,
  Save,
  Star,
  Target,
  Trash2,
} from 'lucide-react'

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
import type { TutorialAudienceOption } from '../../types/tutorials'
import { BadgePicker, buildBadgePresets, type BadgeDraft } from '../Badges/BadgePicker'
import { BadgeMedallion } from '../Badges/BadgeMedallion'
import { DEFAULT_PROGRAM_BADGE } from '../Badges/badgeCatalog'
import ReadinessChecklist from './ReadinessChecklist'
import { LOW_PASSING_SCORE, isReady, programIssues } from './inductionReadiness'
import styles from './InductionSettings.module.css'

const STEPS = ['Datos', 'Bloques', 'Empresas', 'Revisar']

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
  /** Lleva a la biblioteca de bloques cuando está vacía. */
  onGoToBlocks?: () => void
}

/**
 * Editor de un programa, como asistente de cuatro pasos: datos, secuencia de
 * bloques, empresas que lo reciben y revisión (reglas, estado, certificado e
 * insignia). Todo se guarda de una vez: el programa, luego su secuencia y
 * luego sus empresas.
 */
export default function InductionProgramEditor({
  programId,
  library,
  companies,
  allPrograms = [],
  onSaved,
  onBack,
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
  const [companySearch, setCompanySearch] = useState('')
  const [badge, setBadge] = useState<BadgeDraft>({ title: '', ...DEFAULT_PROGRAM_BADGE })
  const [customizingBadge, setCustomizingBadge] = useState(false)
  const [templateId, setTemplateId] = useState<number>(0)
  const [templates, setTemplates] = useState<CertificateTemplate[]>([])
  const [issued, setIssued] = useState<ProgramCertificates | null>(null)
  const [saving, setSaving] = useState(false)
  // Uno nuevo empieza por el nombre; uno existente, por el resumen, desde
  // donde se salta a lo que haya que cambiar.
  const [step, setStep] = useState(programId === null ? 0 : 3)

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

  const handleSave = async () => {
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
      const saved = await inductionService.setProgramCompanies(base.id, companyIds)
      success(programId === null ? 'Programa creado.' : 'Programa guardado.')
      onSaved(saved)
      // Al crear, de vuelta a la lista: el programa ya aparece ahí.
      if (programId === null) onBack()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar el programa.')
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <p className={styles.muted}>Cargando programa...</p>

  const sequence = blockIds.map((id) => blockById.get(id)).filter((b): b is InductionBlock => !!b)
  // Lo que falta, calculado sobre el borrador: cambia mientras se edita.
  const issues = programIssues({
    isActive,
    isDefault,
    defaultPassingScore: defaultPassing,
    blocks: sequence,
    companyCount: companyIds.length,
    hasCertificate: templateId > 0,
  })
  const chosenTemplate = templates.find((t) => t.id === templateId) ?? null
  const blocksReady = sequence.length > 0 && sequence.every((b) => b.question_count > 0)
  const audience = isDefault
    ? companyIds.length > 0
      ? `Por defecto + ${companyIds.length} ${companyIds.length === 1 ? 'empresa' : 'empresas'}`
      : 'Todas las empresas sin asignación'
    : companyIds.length > 0
      ? `${companyIds.length} ${companyIds.length === 1 ? 'empresa' : 'empresas'}`
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
  const stepDone = [name.trim() !== '', blocksReady, isDefault || companyIds.length > 0, isReady(issues)]
  const stepSub = [
    name.trim() || 'Nombre y descripción',
    `${blockIds.length} ${blockIds.length === 1 ? 'bloque' : 'bloques'}`,
    audience,
    isReady(issues) ? 'Todo listo' : 'Con avisos',
  ]
  const badgeTitle = badge.title.trim() || name.trim() || 'Insignia del programa'

  return (
    <div>
      <div className={styles.editorHead}>
        <button type="button" className={styles.backBtn} onClick={onBack}>
          <ArrowLeft size={14} /> Programas
        </button>
        <h3 className={styles.keyTitle}>{programId === null ? 'Nuevo programa' : name || 'Programa'}</h3>
        {wasDefault && <span className={styles.tagPrimary}>Por defecto</span>}
      </div>

      <ol className={styles.wizard} aria-label="Pasos del programa">
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
                          {b?.tutorial_title || 'Sin video'} ·{' '}
                          <span style={noQuestions ? { color: '#b91c1c', fontWeight: 600 } : undefined}>
                            {b?.question_count ?? 0} preguntas
                          </span>{' '}
                          ·{' '}
                          <span style={passing < LOW_PASSING_SCORE ? { color: '#b45309', fontWeight: 600 } : undefined}>
                            mínimo {passing}%
                          </span>
                          {b?.tutorial_visible && ' · video visible en Novedades'}
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
                        {b.tutorial_title || 'Sin video'} · {b.question_count}{' '}
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

      {/* --- 3. Empresas --- */}
      <div className={styles.section} hidden={step !== 2}>
        <h3 className={styles.wizardTitle}>¿Quién lo recibe?</h3>
        <p className={styles.wizardIntro}>
          Quien se contrate en las empresas elegidas recibe este programa. Una empresa solo puede
          estar en un programa: si ya estaba en otro, pasa a este.
        </p>

        <label className={isDefault ? styles.defaultCardOn : styles.defaultCard}>
          <input
            type="checkbox"
            checked={isDefault}
            // El por defecto no se desmarca desde aquí: se marca otro y este
            // deja de serlo solo.
            disabled={wasDefault}
            onChange={(e) => setIsDefault(e.target.checked)}
          />
          <Star size={18} />
          <span>
            <strong>Programa por defecto</strong>
            <span>
              {wasDefault
                ? 'Lo reciben todas las empresas sin asignación. Para cambiarlo, marca otro programa con la estrella desde la lista.'
                : 'Lo reciben todas las empresas que no tengan un programa asignado. Solo puede haber uno.'}
            </span>
          </span>
        </label>

        <div className={styles.companyHead}>
          <span className={styles.blockPreviewLabel}>
            {isDefault ? 'Además, estas empresas' : 'Empresas'} · {companyIds.length} elegidas
          </span>
          <input
            type="text"
            className={styles.searchInput}
            placeholder="Buscar empresa..."
            value={companySearch}
            onChange={(e) => setCompanySearch(e.target.value)}
            style={{ margin: 0 }}
          />
        </div>
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

      {/* --- 4. Revisar --- */}
      <div className={styles.section} hidden={step !== 3}>
        <h3 className={styles.wizardTitle}>Revisa y {programId === null ? 'crea' : 'guarda'} el programa</h3>
        <p className={styles.wizardIntro}>
          Las reglas, el estado y el certificado se ajustan aquí mismo.
        </p>

        <div className={styles.reviewSplit}>
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

          <div className={styles.reviewAside}>
            {/* --- Insignia: como la verá el profesional; se edita aparte --- */}
            <aside className={styles.badgeCard} aria-label="Insignia del programa">
              <span className={styles.blockPreviewLabel}>Insignia que gana</span>
              <BadgeMedallion icon={badge.icon} color={badge.color} size="lg" />
              <strong className={styles.badgeCardTitle}>{badgeTitle}</strong>
              <span className={styles.badgeCardHint}>
                Se gana al completar todos los bloques. Además, «A la primera» si no falla ningún
                intento e «Impecable» si saca 100% en todo.
              </span>
              <button
                type="button"
                className={styles.ghostBtnSm}
                aria-expanded={customizingBadge}
                onClick={() => setCustomizingBadge((v) => !v)}
              >
                <Palette size={14} /> {customizingBadge ? 'Listo' : 'Personalizar'}
              </button>
            </aside>

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
        </div>

        {customizingBadge && (
          <div className={styles.badgeEditor}>
            <BadgePicker value={badge} fallbackTitle={name} presets={presets} onChange={setBadge} />
          </div>
        )}

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

      <div className={styles.stickyBar}>
        <span className={styles.muted}>
          Paso {step + 1} de {STEPS.length}
        </span>
        {step === 0 ? (
          <button type="button" className={styles.ghostBtn} onClick={onBack} disabled={saving}>
            {programId === null ? 'Cancelar' : 'Volver'}
          </button>
        ) : (
          <button type="button" className={styles.ghostBtn} onClick={() => goTo(step - 1)} disabled={saving}>
            <ArrowLeft size={16} /> Atrás
          </button>
        )}
        {/* Un programa ya creado se puede guardar desde cualquier paso. */}
        {programId !== null && !isLast && (
          <button type="button" className={styles.ghostBtn} disabled={saving} onClick={handleSave}>
            <Save size={16} /> {saving ? 'Guardando...' : 'Guardar'}
          </button>
        )}
        {isLast ? (
          <button type="button" className={styles.saveBtn} disabled={saving} onClick={handleSave}>
            <Save size={16} /> {saving ? 'Guardando...' : programId === null ? 'Crear programa' : 'Guardar programa'}
          </button>
        ) : (
          <button type="button" className={styles.saveBtn} onClick={() => goTo(step + 1)}>
            Siguiente <ArrowRight size={16} />
          </button>
        )}
      </div>
    </div>
  )
}
