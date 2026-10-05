import { useRef, useState } from 'react'
import { ArrowLeft, Save, SlidersHorizontal } from 'lucide-react'

import { Select } from '../ui'
import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionBlock, type InductionProgram } from '../../services/induction.service'
import { surveyService } from '../../services/surveyService'
import type { Tutorial } from '../../types/tutorials'
import InductionQuizBuilder, { type QuizBuilderHandle } from './InductionQuizBuilder'
import ReadinessChecklist from './ReadinessChecklist'
import { blockIssues } from './inductionReadiness'
import { BadgePicker, buildBadgePresets, type BadgeDraft } from '../Badges/BadgePicker'
import { DEFAULT_BLOCK_BADGE } from '../Badges/badgeCatalog'
import styles from './InductionSettings.module.css'

interface Props {
  /** null = bloque nuevo. */
  block: InductionBlock | null
  /** Novedades de tipo video, elegibles como material del bloque. */
  tutorials: Tutorial[]
  /** Mínimo que se aplica cuando el bloque no trae uno propio (solo informativo). */
  fallbackPassingScore: number
  /** Biblioteca completa y programas, para copiar una insignia ya definida. */
  allBlocks?: InductionBlock[]
  allPrograms?: InductionProgram[]
  onSaved: (block: InductionBlock) => void
  onBack: () => void
}

/**
 * Editor de un bloque: nombre, video (de Novedades) y su cuestionario; el
 * mínimo propio y la insignia van plegados en opciones avanzadas. Un bloque
 * nuevo nace con un cuestionario vacío creado aquí mismo, y un solo botón
 * guarda el bloque y sus preguntas.
 */
export default function InductionBlockEditor({
  block,
  tutorials,
  fallbackPassingScore,
  allBlocks = [],
  allPrograms = [],
  onSaved,
  onBack,
}: Props) {
  const { success, error: showError } = useNotification()

  const [current, setCurrent] = useState<InductionBlock | null>(block)
  const [name, setName] = useState(block?.name ?? '')
  const [description, setDescription] = useState(block?.description ?? '')
  const [tutorialId, setTutorialId] = useState<number>(block?.tutorial_id ?? 0)
  // Cadena vacía = usa el del programa. Se guarda como texto para permitir
  // borrar el campo sin que salte a 0.
  const [passing, setPassing] = useState<string>(
    block?.passing_score === null || block?.passing_score === undefined ? '' : String(block.passing_score)
  )
  const [badge, setBadge] = useState<BadgeDraft>({
    title: block?.badge_title || '',
    icon: block?.badge_icon || DEFAULT_BLOCK_BADGE.icon,
    color: block?.badge_color || DEFAULT_BLOCK_BADGE.color,
  })
  const presets = buildBadgePresets(allBlocks, allPrograms, block ? { kind: 'block', id: block.id } : undefined)
  const [saving, setSaving] = useState(false)
  const quizRef = useRef<QuizBuilderHandle>(null)

  const passingValue = passing.trim() === '' ? null : Number(passing)
  const effectivePassing = passingValue ?? fallbackPassingScore

  const videoOptions = tutorials.filter((t) => (t.content_type || 'video') === 'video')
  const chosenVideo = videoOptions.find((t) => t.id === tutorialId)
  // Lo que falta, con el borrador sin guardar (las preguntas cuentan las guardadas).
  const issues = current
    ? blockIssues(
        {
          name,
          question_count: current.question_count,
          passing_score: passingValue,
          tutorial_id: tutorialId || null,
          tutorial_visible: chosenVideo?.is_active ?? false,
        },
        fallbackPassingScore
      )
    : []

  const handleSave = async () => {
    if (!name.trim()) {
      showError('El bloque necesita un nombre.')
      return
    }
    if (passingValue !== null && (Number.isNaN(passingValue) || passingValue < 0 || passingValue > 100)) {
      showError('El mínimo aprobatorio debe estar entre 0 y 100.')
      return
    }
    const quizProblem = quizRef.current?.validate()
    if (quizProblem) {
      showError(quizProblem)
      return
    }
    setSaving(true)
    try {
      let surveyId = current?.survey_id
      if (!surveyId) {
        // Cuestionario propio del bloque. No se envía por correo ni por
        // campanita: se responde desde la landing.
        const created = await surveyService.createSurvey({
          title: `Cuestionario: ${name.trim()}`,
          description: 'Responde estas preguntas para completar este bloque de tu inducción.',
          status: 'active',
          kind: 'induction',
          passing_score: effectivePassing,
          send_by_email: false,
          send_by_inapp: false,
          recipient_list: '[]',
          questions: [],
        })
        surveyId = created.id as number
      }
      const input = {
        name: name.trim(),
        description: description.trim(),
        tutorial_id: tutorialId || null,
        survey_id: surveyId,
        passing_score: passingValue,
        badge_title: badge.title.trim(),
        badge_icon: badge.icon,
        badge_color: badge.color,
      }
      let saved = current
        ? await inductionService.updateBlock(current.id, input)
        : await inductionService.createBlock(input)
      // Las preguntas se guardan en el mismo clic (con el mínimo ya resuelto).
      if (current && quizRef.current) {
        const count = await quizRef.current.save()
        saved = { ...saved, question_count: count }
      }
      setCurrent(saved)
      success(current ? 'Bloque guardado.' : 'Bloque creado. Ahora agrégale preguntas.')
      onSaved(saved)
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar el bloque.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <div className={styles.editorHead}>
        <button type="button" className={styles.backBtn} onClick={onBack}>
          <ArrowLeft size={14} /> Bloques
        </button>
        <h3 className={styles.keyTitle}>{current ? current.name : 'Nuevo bloque'}</h3>
        {current && current.program_names.length > 0 && (
          <span className={styles.tag}>En uso: {current.program_names.join(', ')}</span>
        )}
      </div>

      {/* --- 1. Video --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>1</span>
          <h3 className={styles.keyTitle}>Nombre y video</h3>
        </div>
        <p className={styles.sectionIntro}>
          Un bloque es un video más su cuestionario. Se arma una vez y se reutiliza en los
          programas que haga falta.
        </p>

        <div className={styles.grid}>
          <div className={styles.field}>
            <label htmlFor="block-name">Nombre del bloque</label>
            <input
              id="block-name"
              type="text"
              placeholder="Ej. Bienvenida y cultura"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus={!current}
            />
          </div>

          <div className={styles.field}>
            <label>Video (de Novedades)</label>
            <Select
              fullWidth
              value={tutorialId}
              onChange={(v) => setTutorialId(Number(v) || 0)}
              options={[
                { value: 0, label: 'Sin video (solo cuestionario)' },
                // La landing de inducción reproduce un video: una novedad de
                // imagen o de texto no sirve como material aquí.
                ...videoOptions.map((t) => ({
                  value: t.id,
                  label: t.is_active ? `${t.title} (visible en Novedades)` : t.title,
                })),
              ]}
            />
          </div>

          <div className={`${styles.field} ${styles.fieldWide}`}>
            <label htmlFor="block-description">Descripción (opcional)</label>
            <textarea
              id="block-description"
              placeholder="Qué aprende el profesional en este bloque."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
        </div>

        {videoOptions.length === 0 ? (
          <p className={styles.hint}>
            No hay novedades de tipo video todavía. Crea una desde la pestaña Novedades (puede
            quedar oculta) y vuelve a elegirla aquí.
          </p>
        ) : (
          chosenVideo?.is_active && (
            <p className={styles.hint}>
              Este video está visible en Novedades, así que se anuncia a toda su audiencia. Para
              inducción conviene ocultarlo desde Novedades: el bloque lo reproduce igual.
            </p>
          )
        )}
      </div>

      {/* --- 2. Cuestionario --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>2</span>
          <h3 className={styles.keyTitle}>Cuestionario</h3>
          {current && (
            <span className={current.question_count > 0 ? styles.tagOk : styles.tagWarn}>
              {current.question_count} {current.question_count === 1 ? 'pregunta' : 'preguntas'}
            </span>
          )}
        </div>
        {current ? (
          <InductionQuizBuilder ref={quizRef} surveyId={current.survey_id} passingScore={effectivePassing} />
        ) : (
          <p className={styles.sectionIntro}>
            Pulsa «Crear bloque» abajo: el cuestionario se crea con él y aquí mismo podrás
            agregarle las preguntas.
          </p>
        )}
      </div>

      {/* --- Opciones avanzadas: casi nunca hace falta tocarlas --- */}
      <details className={styles.advanced} open={passingValue !== null || undefined}>
        <summary>
          <SlidersHorizontal size={15} /> Opciones avanzadas
          <span className={styles.advancedHint}>Mínimo propio ({effectivePassing}%) e insignia</span>
        </summary>
        <div className={styles.advancedBody}>
          <div className={styles.field} style={{ maxWidth: 320, marginTop: 14 }}>
            <label htmlFor="block-passing">Mínimo aprobatorio propio (%)</label>
            <input
              id="block-passing"
              type="number"
              min={0}
              max={100}
              placeholder={`El del programa (${fallbackPassingScore})`}
              value={passing}
              onChange={(e) => setPassing(e.target.value)}
            />
          </div>
          <p className={styles.hint}>
            Vacío = usa el mínimo del programa. Cámbialo solo si este bloque debe ser más exigente
            o más fácil.
          </p>

          <div className={styles.section}>
            <h3 className={styles.keyTitle} style={{ fontSize: 14, margin: '0 0 4px' }}>
              Insignia del bloque
            </h3>
            <p className={styles.hint} style={{ margin: '0 0 12px' }}>
              Se gana al aprobar este bloque y aparece en el perfil del profesional y en su
              expediente. Si no la cambias, lleva el nombre del bloque.
            </p>
            <BadgePicker value={badge} fallbackTitle={name} presets={presets} onChange={setBadge} />
          </div>
        </div>
      </details>

      {current && (
        <ReadinessChecklist issues={issues} readyText="Este bloque está listo para usarse en un programa." />
      )}

      <div className={styles.stickyBar}>
        <span className={styles.muted}>
          {current ? `Mínimo efectivo: ${effectivePassing}%` : 'El cuestionario se crea al guardar.'}
        </span>
        <button type="button" className={styles.ghostBtn} onClick={onBack} disabled={saving}>
          {current ? 'Volver' : 'Cancelar'}
        </button>
        <button type="button" className={styles.saveBtn} disabled={saving} onClick={handleSave}>
          <Save size={16} /> {saving ? 'Guardando...' : current ? 'Guardar bloque' : 'Crear bloque'}
        </button>
      </div>
    </div>
  )
}
