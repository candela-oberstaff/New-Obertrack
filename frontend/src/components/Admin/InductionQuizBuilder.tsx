import { useCallback, useEffect, useMemo, useState } from 'react'
import { Save, Plus, Trash2, ListChecks, Type, Star, ChevronDown, ChevronRight } from 'lucide-react'

import { Select } from '../ui'
import { useNotification } from '../../context/NotificationContext'
import { surveyService, type Survey, type SurveyQuestion } from '../../services/surveyService'
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

interface Props {
  /** Cuestionario (kind induction) del bloque. */
  surveyId: number
  /** Mínimo aprobatorio con el que se guarda el registro del cuestionario. */
  passingScore: number
  /** Avisa cuando cambia el número de preguntas guardadas (para la biblioteca). */
  onSaved?: (questionCount: number) => void
}

/**
 * Constructor del cuestionario calificado de un bloque: preguntas con clave de
 * respuesta y peso. Se arma aquí mismo para no obligar a saltar al módulo de
 * Encuestas y volver.
 */
export default function InductionQuizBuilder({ surveyId, passingScore, onSaved }: Props) {
  const { success, error: showError } = useNotification()

  const [quiz, setQuiz] = useState<Survey | null>(null)
  const [loadingQuiz, setLoadingQuiz] = useState(true)
  const [savingQuiz, setSavingQuiz] = useState(false)
  // Acordeón: una pregunta abierta a la vez para que la lista no crezca sin fin.
  const [openIndex, setOpenIndex] = useState<number | null>(null)

  const loadQuiz = useCallback(async () => {
    setLoadingQuiz(true)
    try {
      setQuiz(await surveyService.getSurvey(surveyId))
    } catch {
      setQuiz(null)
    } finally {
      setLoadingQuiz(false)
    }
  }, [surveyId])

  useEffect(() => {
    void loadQuiz()
  }, [loadQuiz])

  const scorableCount = useMemo(
    () =>
      (quiz?.questions ?? []).filter(
        (q) => (q.weight ?? 0) > 0 && (q.correct_answer ?? '').trim() !== ''
      ).length,
    [quiz]
  )

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
        passing_score: passingScore,
      })
      success('Cuestionario guardado.')
      await loadQuiz()
      onSaved?.(questions.length)
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar el cuestionario.')
    } finally {
      setSavingQuiz(false)
    }
  }

  if (loadingQuiz) return <p className={styles.muted}>Cargando preguntas...</p>
  if (!quiz) return <p className={styles.muted}>No se pudo cargar el cuestionario.</p>

  const total = quiz.questions?.length ?? 0

  return (
    <>
      <p className={styles.intro}>
        Solo puntúan las preguntas con respuesta correcta y peso mayor a cero. El puntaje se
        calcula sobre la suma de los pesos, no sobre el número de preguntas.
      </p>

      <div className={styles.field} style={{ marginBottom: 18, maxWidth: 640 }}>
        <label>Título del cuestionario</label>
        <input
          type="text"
          value={quiz.title}
          onChange={(e) => setQuiz({ ...quiz, title: e.target.value })}
        />
      </div>

      <div className={total === 0 || scorableCount === 0 ? styles.danger : styles.ok}>
        {total === 0
          ? 'El cuestionario no tiene preguntas todavía.'
          : scorableCount === 0
            ? 'Ninguna pregunta puntúa. Sin clave de respuestas, todo el mundo aprueba este bloque automáticamente.'
            : `${scorableCount} de ${total} preguntas puntúan.`}
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
                  aria-label={isOpen ? 'Plegar pregunta' : 'Desplegar pregunta'}
                  onClick={() => setOpenIndex(isOpen ? null : index)}
                >
                  {isOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                  <span className={styles.questionNum}>{index + 1}</span>
                </button>
                {/* El enunciado se escribe aquí mismo. Tocarlo despliega la
                    pregunta, así opciones y clave quedan a mano. */}
                <input
                  type="text"
                  className={styles.inlineQuestion}
                  placeholder="Escribe la pregunta..."
                  aria-label={`Enunciado de la pregunta ${index + 1}`}
                  value={q.text}
                  autoFocus={isOpen && q.text === ''}
                  onFocus={() => setOpenIndex(index)}
                  onChange={(e) => updateQuestion(index, { text: e.target.value })}
                />
                <span className={styles.tag}>{TYPE_LABEL[q.type] ?? q.type}</span>
                {scores ? (
                  <span className={styles.tagOk}>Peso {q.weight}</span>
                ) : (
                  <span className={styles.tagWarn}>No puntúa</span>
                )}
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
          <button key={type} type="button" className={styles.ghostBtnSm} onClick={() => addQuestion(type)}>
            <Icon size={14} /> {label}
          </button>
        ))}
      </div>

      <div className={styles.actions}>
        <button type="button" className={styles.saveBtn} disabled={savingQuiz} onClick={handleSaveQuiz}>
          <Save size={16} /> {savingQuiz ? 'Guardando...' : 'Guardar cuestionario'}
        </button>
      </div>
      <p className={styles.hint}>
        Las respuestas correctas nunca se envían al navegador de quien responde: el puntaje se
        calcula en el servidor.
      </p>
    </>
  )
}
