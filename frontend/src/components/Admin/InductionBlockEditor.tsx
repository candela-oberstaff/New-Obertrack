import { useState } from 'react'
import { ArrowLeft, Save, Video } from 'lucide-react'

import { Select } from '../ui'
import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionBlock, type InductionProgram } from '../../services/induction.service'
import { surveyService } from '../../services/surveyService'
import type { Tutorial } from '../../types/tutorials'
import InductionQuizBuilder from './InductionQuizBuilder'
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
 * Editor de un bloque: nombre, video (de Novedades), mínimo propio y su
 * cuestionario. Un bloque nuevo nace con un cuestionario vacío creado aquí
 * mismo, para que el constructor de preguntas aparezca sin pasos intermedios.
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

  const passingValue = passing.trim() === '' ? null : Number(passing)
  const effectivePassing = passingValue ?? fallbackPassingScore

  const videoOptions = tutorials.filter((t) => (t.content_type || 'video') === 'video')

  const handleSave = async () => {
    if (!name.trim()) {
      showError('El bloque necesita un nombre.')
      return
    }
    if (passingValue !== null && (Number.isNaN(passingValue) || passingValue < 0 || passingValue > 100)) {
      showError('El mínimo aprobatorio debe estar entre 0 y 100.')
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
      const saved = current
        ? await inductionService.updateBlock(current.id, input)
        : await inductionService.createBlock(input)
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

      {/* --- 1. Datos --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>1</span>
          <h3 className={styles.keyTitle}>Datos del bloque</h3>
        </div>
        <p className={styles.sectionIntro}>
          Un bloque es un video más su cuestionario. Se arma una vez y se reutiliza en los
          programas que haga falta.
        </p>

        <div className={styles.grid}>
          <div className={`${styles.field} ${styles.fieldWide}`}>
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

          <div className={`${styles.field} ${styles.fieldWide}`}>
            <label htmlFor="block-description">Descripción (opcional)</label>
            <textarea
              id="block-description"
              placeholder="Qué aprende el profesional en este bloque."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
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
                  label: t.is_active ? t.title : `${t.title} (oculta)`,
                })),
              ]}
            />
          </div>

          <div className={styles.field}>
            <label htmlFor="block-passing">Mínimo aprobatorio (%)</label>
            <input
              id="block-passing"
              type="number"
              min={0}
              max={100}
              placeholder={`Del programa (${fallbackPassingScore})`}
              value={passing}
              onChange={(e) => setPassing(e.target.value)}
            />
          </div>
        </div>

        <p className={styles.hint}>
          <Video size={13} style={{ verticalAlign: -2, marginRight: 4 }} />
          {videoOptions.length === 0
            ? 'No hay novedades de tipo video todavía. Crea una desde la pestaña Novedades (puede quedar oculta) y vuelve a elegirla aquí.'
            : 'Una novedad publicada como visible se anuncia a toda su audiencia. Para un video de inducción conviene dejarla oculta: el bloque la reproduce igual.'}{' '}
          Si el mínimo se deja vacío, se usa el del programa.
        </p>
      </div>

      {/* --- 2. Insignia --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>2</span>
          <h3 className={styles.keyTitle}>Insignia</h3>
        </div>
        <p className={styles.sectionIntro}>
          Es lo que gana el profesional al aprobar este bloque. Aparece en su perfil, en su
          expediente y en la ficha que ve su empresa.
        </p>
        <BadgePicker value={badge} fallbackTitle={name} presets={presets} onChange={setBadge} />
      </div>

      {/* --- 3. Cuestionario --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>3</span>
          <h3 className={styles.keyTitle}>Cuestionario del bloque</h3>
          {current && (
            <span className={current.question_count > 0 ? styles.tagOk : styles.tagWarn}>
              {current.question_count} {current.question_count === 1 ? 'pregunta' : 'preguntas'}
            </span>
          )}
        </div>
        {current ? (
          <InductionQuizBuilder
            surveyId={current.survey_id}
            passingScore={effectivePassing}
            onSaved={(count) => {
              const updated = { ...current, question_count: count }
              setCurrent(updated)
              onSaved(updated)
            }}
          />
        ) : (
          <p className={styles.sectionIntro}>
            Primero guarda el bloque; el cuestionario se crea con él y aquí podrás agregarle preguntas.
          </p>
        )}
      </div>

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
