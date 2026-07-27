import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  GraduationCap,
  Save,
  AlertTriangle,
  Plus,
  Trash2,
  ListChecks,
  Type,
  Star,
  ChevronDown,
  ChevronRight,
} from 'lucide-react'

import { Select } from '../ui'
import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionConfig } from '../../services/induction.service'
import { surveyService, type Survey, type SurveyQuestion } from '../../services/surveyService'
import { tutorialService } from '../../services/tutorial.service'
import type { Tutorial } from '../../types/tutorials'
import styles from './InductionSettings.module.css'

/**
 * Tipos de pregunta soportados por la landing de inducción. Deliberadamente son
 * solo tres: la landing pública renderiza estos y ningún otro, así que ofrecer
 * más aquí produciría cuestionarios que el profesional no podría responder.
 */
const QUESTION_TYPES = [
  { type: 'choice' as const, label: 'Opción múltiple', icon: ListChecks },
  { type: 'text' as const, label: 'Respuesta escrita', icon: Type },
  { type: 'rating' as const, label: 'Escala 1-5', icon: Star },
]

const TYPE_LABEL: Record<string, string> = {
  choice: 'Opción múltiple',
  text: 'Respuesta escrita',
  rating: 'Escala 1-5',
}

function parseOptions(raw?: string): string[] {
  try {
    const parsed = JSON.parse(raw || '[]')
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

/**
 * Configuración de la inducción del profesional recién contratado.
 *
 * Reúne en una sola pestaña las piezas que antes vivían en módulos distintos:
 * el video (Novedades), el cuestionario con su clave de respuestas y las reglas
 * del portero. El cuestionario se arma aquí mismo para no obligar a saltar al
 * módulo de Encuestas y volver.
 *
 * Solo debe renderizarse para superadmin: es quien puede guardar en el backend.
 */
export default function InductionSettings() {
  const { success, error: showError } = useNotification()

  const [config, setConfig] = useState<InductionConfig | null>(null)
  const [surveys, setSurveys] = useState<Survey[]>([])
  const [tutorials, setTutorials] = useState<Tutorial[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  // Borrador editable del cuestionario elegido.
  const [quiz, setQuiz] = useState<Survey | null>(null)
  const [loadingQuiz, setLoadingQuiz] = useState(false)
  const [savingQuiz, setSavingQuiz] = useState(false)
  const [creating, setCreating] = useState(false)
  // Acordeón: una pregunta abierta a la vez para que la lista no crezca sin fin.
  const [openIndex, setOpenIndex] = useState<number | null>(null)
  // La sección del cuestionario también se plega. Arranca abierta solo si aún no
  // hay uno configurado (ahí el botón de crear es la acción principal).
  const [quizOpen, setQuizOpen] = useState(false)

  const refreshSurveys = useCallback(async () => {
    const list = await surveyService.getSurveys().catch(() => [])
    setSurveys(Array.isArray(list) ? list : [])
  }, [])

  useEffect(() => {
    const load = async () => {
      try {
        const [cfg, surveyList, tutorialList] = await Promise.all([
          inductionService.getConfig(),
          surveyService.getSurveys().catch(() => []),
          tutorialService.getAll().catch(() => []),
        ])
        setConfig(cfg)
        setQuizOpen(!cfg.survey_id)
        setSurveys(Array.isArray(surveyList) ? surveyList : [])
        setTutorials(Array.isArray(tutorialList) ? tutorialList : [])
      } catch {
        showError('No se pudo cargar la configuración de inducción.')
      } finally {
        setLoading(false)
      }
    }
    void load()
  }, [showError])

  const loadQuiz = useCallback(async (surveyId: number | null | undefined) => {
    if (!surveyId) {
      setQuiz(null)
      return
    }
    setLoadingQuiz(true)
    try {
      setQuiz(await surveyService.getSurvey(surveyId))
    } catch {
      setQuiz(null)
    } finally {
      setLoadingQuiz(false)
    }
  }, [])

  useEffect(() => {
    void loadQuiz(config?.survey_id)
  }, [config?.survey_id, loadQuiz])

  const scorableCount = useMemo(
    () =>
      (quiz?.questions ?? []).filter(
        (q) => (q.weight ?? 0) > 0 && (q.correct_answer ?? '').trim() !== ''
      ).length,
    [quiz]
  )

  const persistConfig = async (patch: Partial<InductionConfig>) => {
    if (!config) return null
    const saved = await inductionService.saveConfig({
      tutorial_id: config.tutorial_id || null,
      survey_id: config.survey_id || null,
      passing_score: config.passing_score,
      max_attempts: config.max_attempts,
      invite_ttl_days: config.invite_ttl_days,
      is_active: config.is_active,
      ...patch,
    })
    setConfig(saved)
    return saved
  }

  const handleSaveConfig = async () => {
    setSaving(true)
    try {
      await persistConfig({})
      success('Configuración de inducción guardada.')
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar la configuración.')
    } finally {
      setSaving(false)
    }
  }

  // Crea un cuestionario vacío y lo deja enlazado a la inducción en un solo paso.
  const handleCreateQuiz = async () => {
    setCreating(true)
    try {
      const created = await surveyService.createSurvey({
        title: 'Cuestionario de inducción',
        description: 'Responde estas preguntas para completar tu inducción.',
        status: 'active',
        kind: 'induction',
        passing_score: config?.passing_score ?? 70,
        // No se envía por correo ni por campanita: se responde desde la landing.
        send_by_email: false,
        send_by_inapp: false,
        recipient_list: '[]',
        questions: [],
      })
      await refreshSurveys()
      await persistConfig({ survey_id: created.id })
      success('Cuestionario creado. Agrégale preguntas abajo.')
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo crear el cuestionario.')
    } finally {
      setCreating(false)
    }
  }

  const updateQuestion = (index: number, patch: Partial<SurveyQuestion>) => {
    if (!quiz?.questions) return
    setQuiz({
      ...quiz,
      questions: quiz.questions.map((q, i) => (i === index ? { ...q, ...patch } : q)),
    })
  }

  const addQuestion = (type: 'choice' | 'text' | 'rating') => {
    if (!quiz) return
    const questions = quiz.questions ?? []
    setQuiz({
      ...quiz,
      questions: [
        ...questions,
        {
          text: '',
          type,
          options: type === 'choice' ? JSON.stringify(['', '']) : '',
          is_required: true,
          order_index: questions.length,
          correct_answer: '',
          weight: 1,
        },
      ],
    })
    // La recién creada se abre: hay que escribirle el enunciado.
    setOpenIndex(questions.length)
  }

  const removeQuestion = (index: number) => {
    if (!quiz?.questions) return
    setQuiz({
      ...quiz,
      questions: quiz.questions
        .filter((_, i) => i !== index)
        .map((q, i) => ({ ...q, order_index: i })),
    })
    // Reajusta el acordeón: los índices posteriores se corren una posición.
    setOpenIndex((cur) => {
      if (cur === null) return null
      if (cur === index) return null
      return cur > index ? cur - 1 : cur
    })
  }

  const setOptions = (index: number, options: string[]) => {
    // Si la respuesta correcta apuntaba a una opción que ya no existe, se limpia
    // para no dejar una pregunta imposible de acertar.
    const q = quiz?.questions?.[index]
    const stillValid = q?.correct_answer && options.includes(q.correct_answer)
    updateQuestion(index, {
      options: JSON.stringify(options),
      ...(stillValid ? {} : { correct_answer: '' }),
    })
  }

  const handleSaveQuiz = async () => {
    if (!quiz?.id) return

    const questions = (quiz.questions ?? []).map((q, i) => ({ ...q, order_index: i }))
    if (questions.some((q) => !q.text.trim())) {
      showError('Hay preguntas sin enunciado.')
      return
    }

    setSavingQuiz(true)
    try {
      await surveyService.updateSurvey(quiz.id, {
        ...quiz,
        questions,
        kind: 'induction',
        passing_score: config?.passing_score ?? quiz.passing_score,
      })
      await refreshSurveys()
      success('Cuestionario guardado.')
      await loadQuiz(quiz.id)
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar el cuestionario.')
    } finally {
      setSavingQuiz(false)
    }
  }

  if (loading || !config) {
    return (
      <div className={styles.panel}>
        <p className={styles.muted}>Cargando inducción...</p>
      </div>
    )
  }

  const canActivate = Boolean(config.survey_id)

  return (
    <div className={styles.panel}>
      <div className={styles.head}>
        <div className={styles.icon}>
          <GraduationCap size={22} />
        </div>
        <div>
          <h2 className={styles.title}>Inducción de nuevos profesionales</h2>
          <p className={styles.intro}>
            Quien llega contratado desde Obersuite recibe un enlace, ve el video y responde el
            cuestionario. Si aprueba, se le habilita el acceso; si agota los intentos, se abre una
            alerta en Soporte.
          </p>
        </div>
      </div>

      {!canActivate && (
        <div className={styles.warn}>
          <AlertTriangle size={16} style={{ flexShrink: 0, marginTop: 1 }} />
          <span>
            Crea o elige un cuestionario para poder activar la inducción. Mientras esté apagada, los
            profesionales contratados reciben acceso directo, como hasta ahora.
          </span>
        </div>
      )}

      <div className={styles.toggleRow}>
        <span className={styles.toggleLabel}>
          Activar inducción obligatoria
          <span className={styles.toggleHint}>
            Con esto encendido, ningún profesional nuevo entra sin aprobar.
          </span>
        </span>
        <input
          type="checkbox"
          checked={config.is_active}
          disabled={!canActivate}
          onChange={(e) => setConfig({ ...config, is_active: e.target.checked })}
        />
      </div>

      <div className={styles.grid}>
        <div className={styles.field}>
          <label>Video de inducción</label>
          <Select
            fullWidth
            value={config.tutorial_id ?? 0}
            onChange={(v) => setConfig({ ...config, tutorial_id: Number(v) || null })}
            options={[
              { value: 0, label: 'Sin video (solo cuestionario)' },
              ...tutorials.map((t) => ({ value: t.id, label: t.title })),
            ]}
          />
        </div>

        <div className={styles.field}>
          <label>Mínimo aprobatorio (%)</label>
          <input
            type="number"
            min={0}
            max={100}
            value={config.passing_score}
            onChange={(e) => setConfig({ ...config, passing_score: Number(e.target.value) })}
          />
        </div>

        <div className={styles.field}>
          <label>Intentos permitidos</label>
          <input
            type="number"
            min={1}
            max={10}
            value={config.max_attempts}
            onChange={(e) => setConfig({ ...config, max_attempts: Number(e.target.value) })}
          />
        </div>

        <div className={styles.field}>
          <label>Vigencia del enlace (días)</label>
          <input
            type="number"
            min={1}
            max={365}
            value={config.invite_ttl_days}
            onChange={(e) => setConfig({ ...config, invite_ttl_days: Number(e.target.value) })}
          />
        </div>
      </div>

      <div className={styles.actions}>
        <button type="button" className={styles.saveBtn} disabled={saving} onClick={handleSaveConfig}>
          <Save size={16} /> {saving ? 'Guardando...' : 'Guardar configuración'}
        </button>
      </div>

      {/* --- Cuestionario (plegable) --- */}
      <div className={styles.keySection}>
        <button
          type="button"
          className={styles.sectionToggle}
          aria-expanded={quizOpen}
          onClick={() => setQuizOpen((v) => !v)}
        >
          {quizOpen ? <ChevronDown size={18} /> : <ChevronRight size={18} />}
          <h3 className={styles.keyTitle}>Cuestionario</h3>
          {/* Resumen para saber en qué estado está sin desplegar. */}
          {!config.survey_id ? (
            <span className={styles.tagWarn}>Sin configurar</span>
          ) : (
            <>
              <span className={styles.tag}>{quiz?.title ?? '...'}</span>
              {(quiz?.questions?.length ?? 0) === 0 ? (
                <span className={styles.tagWarn}>Sin preguntas</span>
              ) : scorableCount === 0 ? (
                <span className={styles.tagWarn}>Ninguna puntúa</span>
              ) : (
                <span className={styles.tagOk}>
                  {quiz?.questions?.length} preguntas · {scorableCount} puntúan
                </span>
              )}
            </>
          )}
        </button>

        {!quizOpen ? null : (
        <>
        <p className={styles.intro}>
          Solo puntúan las preguntas con respuesta correcta y peso mayor a cero. El puntaje se
          calcula sobre la suma de los pesos, no sobre el número de preguntas.
        </p>

        <div className={styles.quizPicker}>
          <div className={styles.field} style={{ flex: 1, minWidth: 220 }}>
            <label>Cuestionario en uso</label>
            <Select
              fullWidth
              value={config.survey_id ?? 0}
              onChange={(v) => setConfig({ ...config, survey_id: Number(v) || null })}
              options={[
                { value: 0, label: '— Ninguno —' },
                ...surveys.map((s) => ({ value: s.id as number, label: s.title })),
              ]}
            />
          </div>
          <button
            type="button"
            className={styles.ghostBtn}
            disabled={creating}
            onClick={handleCreateQuiz}
          >
            <Plus size={16} /> {creating ? 'Creando...' : 'Crear cuestionario nuevo'}
          </button>
        </div>

        {!config.survey_id ? (
          <p className={styles.muted}>
            Elige uno existente o crea uno nuevo para empezar a agregar preguntas.
          </p>
        ) : loadingQuiz ? (
          <p className={styles.muted}>Cargando preguntas...</p>
        ) : !quiz ? (
          <p className={styles.muted}>No se pudo cargar el cuestionario.</p>
        ) : (
          <>
            <div className={styles.field} style={{ marginBottom: 18 }}>
              <label>Título del cuestionario</label>
              <input
                type="text"
                value={quiz.title}
                onChange={(e) => setQuiz({ ...quiz, title: e.target.value })}
              />
            </div>

            <div className={(quiz.questions?.length ?? 0) === 0 || scorableCount === 0 ? styles.danger : styles.ok}>
              {(quiz.questions?.length ?? 0) === 0
                ? 'El cuestionario no tiene preguntas todavía.'
                : scorableCount === 0
                  ? 'Ninguna pregunta puntúa. Sin clave de respuestas, todo el mundo aprueba automáticamente.'
                  : `${scorableCount} de ${quiz.questions?.length} preguntas puntúan.`}
            </div>

            <div className={styles.questionList}>
              {(quiz.questions ?? []).map((q, index) => {
                const options = parseOptions(q.options)
                const isOpen = openIndex === index
                const scores = (q.weight ?? 0) > 0 && (q.correct_answer ?? '').trim() !== ''

                return (
                  <div key={q.id ?? `new-${index}`} className={styles.questionCard}>
                    {/* Cabecera plegable: resume la pregunta para poder revisar el
                        cuestionario completo de un vistazo sin desplegar cada una. */}
                    <div className={styles.questionTop}>
                      <button
                        type="button"
                        className={styles.disclosure}
                        aria-expanded={isOpen}
                        onClick={() => setOpenIndex(isOpen ? null : index)}
                      >
                        {isOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                        <span className={styles.questionNum}>{index + 1}</span>
                        <span className={q.text.trim() ? styles.summaryText : styles.summaryEmpty}>
                          {q.text.trim() || 'Pregunta sin enunciado'}
                        </span>
                        <span className={styles.tag}>{TYPE_LABEL[q.type] ?? q.type}</span>
                        {scores ? (
                          <span className={styles.tagOk}>Peso {q.weight}</span>
                        ) : (
                          <span className={styles.tagWarn}>No puntúa</span>
                        )}
                      </button>
                      <button
                        type="button"
                        className={styles.iconBtn}
                        title="Eliminar pregunta"
                        onClick={() => removeQuestion(index)}
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>

                    {!isOpen ? null : (
                    <>
                    <input
                      type="text"
                      className={styles.questionInput}
                      placeholder="Escribe la pregunta..."
                      value={q.text}
                      onChange={(e) => updateQuestion(index, { text: e.target.value })}
                    />

                    {q.type === 'choice' && (
                      <div className={styles.optionsBox}>
                        <label className={styles.smallLabel}>Opciones</label>
                        {options.map((opt, oi) => (
                          <div key={oi} className={styles.optionRow}>
                            <input
                              type="text"
                              placeholder={`Opción ${oi + 1}`}
                              value={opt}
                              onChange={(e) => {
                                const next = [...options]
                                next[oi] = e.target.value
                                setOptions(index, next)
                              }}
                            />
                            <button
                              type="button"
                              className={styles.iconBtn}
                              title="Quitar opción"
                              disabled={options.length <= 2}
                              onClick={() => setOptions(index, options.filter((_, i) => i !== oi))}
                            >
                              <Trash2 size={14} />
                            </button>
                          </div>
                        ))}
                        <button
                          type="button"
                          className={styles.ghostBtnSm}
                          onClick={() => setOptions(index, [...options, ''])}
                        >
                          <Plus size={14} /> Agregar opción
                        </button>
                      </div>
                    )}

                    <div className={styles.questionGrid}>
                      <div className={styles.field}>
                        <label className={styles.smallLabel}>Respuesta correcta</label>
                        {q.type === 'choice' ? (
                          <Select
                            fullWidth
                            value={q.correct_answer ?? ''}
                            onChange={(v) => updateQuestion(index, { correct_answer: String(v) })}
                            options={[
                              { value: '', label: 'No puntúa' },
                              ...options
                                .filter((o) => o.trim() !== '')
                                .map((o) => ({ value: o, label: o })),
                            ]}
                          />
                        ) : q.type === 'rating' ? (
                          <Select
                            fullWidth
                            value={q.correct_answer ?? ''}
                            onChange={(v) => updateQuestion(index, { correct_answer: String(v) })}
                            options={[
                              { value: '', label: 'No puntúa' },
                              ...[1, 2, 3, 4, 5].map((n) => ({ value: String(n), label: String(n) })),
                            ]}
                          />
                        ) : (
                          <input
                            type="text"
                            placeholder="Vacío = no puntúa"
                            value={q.correct_answer ?? ''}
                            onChange={(e) => updateQuestion(index, { correct_answer: e.target.value })}
                          />
                        )}
                      </div>

                      <div className={styles.field}>
                        <label className={styles.smallLabel}>Peso</label>
                        <input
                          type="number"
                          min={0}
                          max={100}
                          value={q.weight ?? 1}
                          onChange={(e) => updateQuestion(index, { weight: Number(e.target.value) })}
                        />
                      </div>
                    </div>
                    </>
                    )}
                  </div>
                )
              })}
            </div>

            <div className={styles.addRow}>
              {QUESTION_TYPES.map(({ type, label, icon: Icon }) => (
                <button
                  key={type}
                  type="button"
                  className={styles.ghostBtnSm}
                  onClick={() => addQuestion(type)}
                >
                  <Icon size={14} /> {label}
                </button>
              ))}
            </div>

            <div className={styles.actions}>
              <button
                type="button"
                className={styles.saveBtn}
                disabled={savingQuiz}
                onClick={handleSaveQuiz}
              >
                <Save size={16} /> {savingQuiz ? 'Guardando...' : 'Guardar cuestionario'}
              </button>
            </div>
            <p className={styles.hint}>
              Las respuestas correctas nunca se envían al navegador de quien responde: el puntaje se
              calcula en el servidor.
            </p>
          </>
        )}
        </>
        )}
      </div>
    </div>
  )
}
