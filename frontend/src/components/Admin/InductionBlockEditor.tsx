import { useCallback, useRef, useState } from 'react'
import { ArrowLeft, ArrowRight, Check, ListChecks, Palette, Play, Save, Target } from 'lucide-react'

import { useNotification } from '../../context/NotificationContext'
import {
  inductionService,
  type InductionBlock,
  type InductionProgram,
  type InductionVideo,
} from '../../services/induction.service'
import { surveyService } from '../../services/surveyService'
import InductionQuizBuilder, { type QuizBuilderHandle } from './InductionQuizBuilder'
import InductionVideoPicker from './InductionVideoPicker'
import ReadinessChecklist from './ReadinessChecklist'
import { blockIssues, isReady } from './inductionReadiness'
import { BadgePicker, buildBadgePresets, type BadgeDraft } from '../Badges/BadgePicker'
import { DEFAULT_BLOCK_BADGE } from '../Badges/badgeCatalog'
import { BadgeMedallion } from '../Badges/BadgeMedallion'
import styles from './InductionSettings.module.css'

interface Props {
  /** null = bloque nuevo. */
  block: InductionBlock | null
  /** Videos de la biblioteca de la inducción, elegibles para el bloque. */
  videos: InductionVideo[]
  /** Mínimo que se aplica cuando el bloque no trae uno propio (solo informativo). */
  fallbackPassingScore: number
  /** Biblioteca completa y programas, para copiar una insignia ya definida. */
  allBlocks?: InductionBlock[]
  allPrograms?: InductionProgram[]
  onSaved: (block: InductionBlock) => void
  onBack: () => void
}

const STEPS = ['Datos', 'Video', 'Cuestionario', 'Revisar']

/**
 * Editor de un bloque, como asistente de cuatro pasos: datos, video (de la
 * biblioteca de la inducción), cuestionario y revisión (con el mínimo propio y la insignia
 * plegados). Las preguntas se escriben antes de crear el bloque: un solo
 * botón crea el cuestionario, el bloque y sus preguntas.
 */
export default function InductionBlockEditor({
  block,
  videos,
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
  const [videoId, setVideoId] = useState<number>(block?.video_id ?? 0)
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
  // Un bloque nuevo empieza por el nombre; uno existente, por sus preguntas,
  // que es lo que más se vuelve a tocar.
  const [step, setStep] = useState(block ? 2 : 0)
  const [questionCount, setQuestionCount] = useState(block?.question_count ?? 0)
  // Sin dato todavía se asume que puntúan: el aviso aparece al cargar las preguntas.
  const [scorableCount, setScorableCount] = useState<number | null>(null)
  const onCountChange = useCallback((total: number, scorable: number) => {
    setQuestionCount(total)
    setScorableCount(scorable)
  }, [])

  const passingValue = passing.trim() === '' ? null : Number(passing)
  const effectivePassing = passingValue ?? fallbackPassingScore

  const [customizingBadge, setCustomizingBadge] = useState(false)
  // Videos agregados a la biblioteca desde este editor, antes de que el panel
  // recargue la suya.
  const [createdVideos, setCreatedVideos] = useState<InductionVideo[]>([])
  const videoOptions = [...videos, ...createdVideos.filter((c) => !videos.some((v) => v.id === c.id))]
  const chosenVideo = videoOptions.find((v) => v.id === videoId)
  // Lo que falta, con el borrador tal como está (preguntas incluidas).
  const issues = blockIssues(
    {
      name,
      question_count: questionCount,
      passing_score: passingValue,
    },
    fallbackPassingScore
  )
  if (questionCount > 0 && scorableCount === 0) {
    issues.unshift({
      level: 'blocker',
      text: 'Ninguna pregunta tiene respuesta correcta, así que todos aprueban sin importar lo que respondan.',
      short: 'Sin respuestas correctas',
      fix: 'En el paso «Cuestionario», abre cada pregunta y elige su respuesta correcta.',
    })
  }

  const handleSave = async () => {
    if (!name.trim()) {
      showError('El bloque necesita un nombre.')
      setStep(0)
      return
    }
    if (passingValue !== null && (Number.isNaN(passingValue) || passingValue < 0 || passingValue > 100)) {
      showError('El mínimo aprobatorio debe estar entre 0 y 100.')
      setStep(3)
      return
    }
    const quizProblem = quizRef.current?.validate()
    if (quizProblem) {
      showError(quizProblem)
      setStep(2)
      return
    }
    setSaving(true)
    let draftCount = 0
    try {
      const quizTitle = `Cuestionario: ${name.trim()}`
      let surveyId = current?.survey_id
      if (!surveyId) {
        // Cuestionario propio del bloque. No se envía por correo ni por
        // campanita: se responde desde la landing.
        const created = await surveyService.createSurvey({
          title: quizTitle,
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
        // Las preguntas del borrador se guardan por la misma vía que las de
        // un bloque existente (la edición del cuestionario).
        const draft = quizRef.current?.questions() ?? []
        if (draft.length > 0) {
          const fresh = await surveyService.getSurvey(surveyId)
          await surveyService.updateSurvey(surveyId, {
            ...fresh,
            questions: draft,
            kind: 'induction',
            passing_score: effectivePassing,
          })
        }
        draftCount = draft.length
      }
      const input = {
        name: name.trim(),
        description: description.trim(),
        video_id: videoId || null,
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
        const count = await quizRef.current.save(quizTitle)
        saved = { ...saved, question_count: count }
      } else if (!current) {
        saved = { ...saved, question_count: draftCount }
      }
      setCurrent(saved)
      success(
        current
          ? 'Bloque guardado.'
          : draftCount > 0
            ? 'Bloque creado con su cuestionario.'
            : 'Bloque creado. Agrégale preguntas para que se pueda aprobar.'
      )
      onSaved(saved)
      // Al crear, de vuelta a la biblioteca: el bloque ya aparece en la lista.
      if (!current) onBack()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar el bloque.')
    } finally {
      setSaving(false)
    }
  }

  // --- Asistente por pasos ---------------------------------------------------
  // Todos los pasos quedan montados (ocultos los que no se ven): así el
  // borrador de preguntas no se pierde al ir y volver.
  const goTo = (target: number) => {
    if (target > 0 && !name.trim()) {
      showError('Primero ponle un nombre al bloque.')
      setStep(0)
      return
    }
    setStep(Math.max(0, Math.min(STEPS.length - 1, target)))
  }
  const isLast = step === STEPS.length - 1
  const stepDone = [name.trim() !== '', videoId > 0, questionCount > 0 && scorableCount !== 0, isReady(issues)]
  const stepSub = [
    name.trim() || 'Nombre y descripción',
    chosenVideo ? chosenVideo.title : 'Sin video',
    `${questionCount} ${questionCount === 1 ? 'pregunta' : 'preguntas'}`,
    isReady(issues) ? 'Todo listo' : 'Con avisos',
  ]

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

      <ol className={styles.wizard} aria-label="Pasos del bloque">
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
        <h3 className={styles.wizardTitle}>¿Cómo se llama este bloque?</h3>
        <p className={styles.wizardIntro}>
          Un bloque es un video más su cuestionario. Se arma una vez y se reutiliza en los
          programas que haga falta.
        </p>
        <div className={styles.wizardSplit}>
          <div>
            <div className={styles.field}>
              <label htmlFor="block-name">Nombre del bloque</label>
              <input
                id="block-name"
                type="text"
                placeholder="Ej. Bienvenida y cultura"
                value={name}
                onChange={(e) => setName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') goTo(1)
                }}
                autoFocus={!current}
              />
            </div>
            <div className={styles.field} style={{ marginTop: 16 }}>
              <label htmlFor="block-description">Descripción (opcional)</label>
              <textarea
                id="block-description"
                placeholder="Qué aprende el profesional en este bloque."
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
          </div>

          {/* Vista previa en vivo: lo que el nombre y la descripción significan
              para quien recorre la inducción. */}
          <aside className={styles.blockPreview} aria-label="Vista previa del bloque">
            <span className={styles.blockPreviewLabel}>Así lo verá el profesional</span>
            <div className={styles.blockPreviewCard}>
              <span className={styles.blockPreviewKicker}>Bloque de tu inducción</span>
              <strong className={styles.blockPreviewName}>{name.trim() || 'Nombre del bloque'}</strong>
              <p className={styles.blockPreviewDesc}>
                {description.trim() || 'Aquí aparece la descripción: qué va a aprender en este bloque.'}
              </p>
              <div className={styles.blockPreviewChips}>
                <span>
                  <Play size={12} /> {chosenVideo ? 'Video' : 'Sin video'}
                </span>
                <span>
                  <ListChecks size={12} /> {questionCount} {questionCount === 1 ? 'pregunta' : 'preguntas'}
                </span>
                <span>
                  <Target size={12} /> Aprueba con {effectivePassing}%
                </span>
              </div>
              <div className={styles.blockPreviewBadge}>
                <BadgeMedallion icon={badge.icon} color={badge.color} size="sm" />
                <span>
                  Al aprobarlo gana <strong>{badge.title.trim() || name.trim() || 'su insignia'}</strong>
                </span>
              </div>
            </div>
          </aside>
        </div>
      </div>

      {/* --- 2. Video --- */}
      <div className={styles.section} hidden={step !== 1}>
        <h3 className={styles.wizardTitle}>Elige el video</h3>
        <p className={styles.wizardIntro}>
          Sale de la biblioteca de videos de la inducción. El profesional lo ve antes de responder
          el cuestionario. Si aún no está, agrégalo con «Nuevo video»; si el bloque es solo
          preguntas, elige «Sin video».
        </p>
        {/* La landing de inducción reproduce un video: una novedad de imagen o
            de texto no sirve como material aquí. */}
        <InductionVideoPicker
          videos={videoOptions}
          value={videoId}
          onChange={setVideoId}
          onCreated={(video) => setCreatedVideos((prev) => [...prev, video])}
        />
      </div>

      {/* --- 3. Cuestionario --- */}
      <div className={styles.section} hidden={step !== 2}>
        <h3 className={styles.wizardTitle}>Arma el cuestionario</h3>
        <p className={styles.wizardIntro}>
          Se responde después del video. Para aprobar hay que llegar al {effectivePassing}%.
        </p>
        {/* En un bloque nuevo las preguntas quedan en borrador y se guardan al crearlo. */}
        <InductionQuizBuilder
          key={current?.survey_id ?? 'nuevo'}
          ref={quizRef}
          surveyId={current?.survey_id ?? null}
          passingScore={effectivePassing}
          onCountChange={onCountChange}
        />
      </div>

      {/* --- 4. Revisar --- */}
      <div className={styles.section} hidden={step !== 3}>
        <h3 className={styles.wizardTitle}>Revisa y {current ? 'guarda' : 'crea'} el bloque</h3>
        <p className={styles.wizardIntro}>Todo se puede ajustar desde aquí antes de {current ? 'guardar' : 'crearlo'}.</p>

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
                <span className={styles.reviewLabel}>Video</span>
                <span className={styles.reviewValue}>
                  {chosenVideo?.title ?? 'Sin video'}
                </span>
                <button type="button" className={styles.linkBtn} onClick={() => goTo(1)}>
                  Cambiar
                </button>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Cuestionario</span>
                <span className={styles.reviewValue}>
                  {questionCount} {questionCount === 1 ? 'pregunta' : 'preguntas'}
                  {questionCount > 0 && scorableCount !== null && (
                    <small>
                      {scorableCount === questionCount
                        ? 'todas puntúan'
                        : `${scorableCount} ${scorableCount === 1 ? 'puntúa' : 'puntúan'}`}
                    </small>
                  )}
                </span>
                <button type="button" className={styles.linkBtn} onClick={() => goTo(2)}>
                  Editar
                </button>
              </div>
              <div className={styles.reviewRow}>
                <span className={styles.reviewLabel}>Mínimo para aprobar</span>
                <span className={styles.reviewValue}>
                  <span className={styles.segmented} role="radiogroup" aria-label="Mínimo para aprobar">
                    <button
                      type="button"
                      role="radio"
                      aria-checked={passingValue === null}
                      className={passingValue === null ? styles.segmentActive : styles.segment}
                      onClick={() => setPassing('')}
                    >
                      El del programa ({fallbackPassingScore}%)
                    </button>
                    <button
                      type="button"
                      role="radio"
                      aria-checked={passingValue !== null}
                      className={passingValue !== null ? styles.segmentActive : styles.segment}
                      onClick={() => passingValue === null && setPassing(String(fallbackPassingScore))}
                    >
                      Propio
                    </button>
                  </span>
                  {passingValue !== null && (
                    <span className={styles.percentInput}>
                      <input
                        id="block-passing"
                        type="number"
                        min={0}
                        max={100}
                        aria-label="Mínimo propio (%)"
                        value={passing}
                        onChange={(e) => setPassing(e.target.value)}
                      />
                      %
                    </span>
                  )}
                </span>
              </div>
            </div>

            <ReadinessChecklist issues={issues} readyText="Este bloque está listo para usarse en un programa." />
          </div>

          {/* --- Insignia: se ve como la verá el profesional; se edita aparte --- */}
          <aside className={styles.badgeCard} aria-label="Insignia del bloque">
            <span className={styles.blockPreviewLabel}>Insignia que gana</span>
            <BadgeMedallion icon={badge.icon} color={badge.color} size="lg" />
            <strong className={styles.badgeCardTitle}>{badge.title.trim() || name.trim() || 'Insignia del bloque'}</strong>
            <span className={styles.badgeCardHint}>
              Se gana al aprobar este bloque y aparece en su perfil y en su expediente.
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
        </div>

        {customizingBadge && (
          <div className={styles.badgeEditor}>
            <BadgePicker value={badge} fallbackTitle={name} presets={presets} onChange={setBadge} />
          </div>
        )}
      </div>

      <div className={styles.stickyBar}>
        <span className={styles.muted}>
          Paso {step + 1} de {STEPS.length}
        </span>
        {step === 0 ? (
          <button type="button" className={styles.ghostBtn} onClick={onBack} disabled={saving}>
            {current ? 'Volver' : 'Cancelar'}
          </button>
        ) : (
          <button type="button" className={styles.ghostBtn} onClick={() => goTo(step - 1)} disabled={saving}>
            <ArrowLeft size={16} /> Atrás
          </button>
        )}
        {/* Un bloque ya creado se puede guardar desde cualquier paso. */}
        {current && !isLast && (
          <button type="button" className={styles.ghostBtn} disabled={saving} onClick={handleSave}>
            <Save size={16} /> {saving ? 'Guardando...' : 'Guardar'}
          </button>
        )}
        {isLast ? (
          <button type="button" className={styles.saveBtn} disabled={saving} onClick={handleSave}>
            <Save size={16} /> {saving ? 'Guardando...' : current ? 'Guardar bloque' : 'Crear bloque'}
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
