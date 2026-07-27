import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  CheckCircle2,
  PlayCircle,
  AlertTriangle,
  LifeBuoy,
  Clock,
  RotateCcw,
  ArrowRight,
} from 'lucide-react'

import {
  inductionService,
  type InductionAnswer,
  type InductionLanding,
  type InductionResult,
} from '../services/induction.service'
import { buildEmbedUrl } from '../components/Tutorials/utils'
import styles from './Induction.module.css'

/** Paso visible de la landing. */
type Step = 'video' | 'quiz' | 'result'

function errorMessage(err: unknown, fallback: string): string {
  const responseError =
    err && typeof err === 'object' && 'response' in err
      ? (err as { response?: { data?: { error?: string } } }).response?.data?.error
      : undefined
  return responseError || fallback
}

/**
 * Landing pública de inducción. Es la primera pantalla que ve un profesional
 * contratado desde Obersuite: mira el video de presentación, responde el
 * cuestionario y, si alcanza el mínimo aprobatorio, se le habilita el acceso a
 * Obertrack.
 *
 * No requiere sesión: el token del enlace es la credencial.
 *
 * Layout: wizard de dos paneles. A la izquierda, panel de marca (logo,
 * bienvenida, progreso vertical); a la derecha, el contenido del paso actual.
 */
export default function Induction() {
  const { token = '' } = useParams<{ token: string }>()

  const [landing, setLanding] = useState<InductionLanding | null>(null)
  const [loadError, setLoadError] = useState('')
  const [loading, setLoading] = useState(true)

  const [step, setStep] = useState<Step>('video')
  const [answers, setAnswers] = useState<Record<number, string>>({})
  const [result, setResult] = useState<InductionResult | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setLoadError('')
    try {
      const data = await inductionService.getLanding(token)
      setLanding(data)
      // Si no hay video configurado, la inducción es solo cuestionario.
      setStep(data.video_url ? 'video' : 'quiz')
    } catch (err) {
      setLoadError(errorMessage(err, 'No pudimos cargar tu inducción. Verifica el enlace de tu correo.'))
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    void load()
  }, [load])

  const embedUrl = useMemo(
    () => (landing?.video_url ? buildEmbedUrl(landing.video_url) : null),
    [landing?.video_url]
  )

  // Pasos del wizard para el stepper vertical. El video es opcional.
  const stepList = useMemo(() => {
    const items: { key: Step; label: string }[] = []
    if (landing?.video_url) items.push({ key: 'video', label: 'Video de inducción' })
    items.push({ key: 'quiz', label: 'Cuestionario' })
    items.push({ key: 'result', label: 'Resultado' })
    return items
  }, [landing?.video_url])

  const currentStep: Step = result ? 'result' : step

  const setAnswer = (questionId: number, value: string) =>
    setAnswers((prev) => ({ ...prev, [questionId]: value }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!landing) return

    setSubmitting(true)
    setSubmitError('')
    try {
      const payload: InductionAnswer[] = landing.questions.map((q) => ({
        question_id: q.id,
        value: answers[q.id] ?? '',
      }))
      const res = await inductionService.submit(token, payload)
      setResult(res)
      setStep('result')
    } catch (err) {
      setSubmitError(errorMessage(err, 'No pudimos enviar tus respuestas. Intenta de nuevo.'))
    } finally {
      setSubmitting(false)
    }
  }

  // Reintento: limpia las respuestas y recarga los intentos restantes.
  const handleRetry = async () => {
    setAnswers({})
    setResult(null)
    setSubmitError('')
    await load()
  }

  // --- Panel de marca (izquierda). showSteps=false en pantallas terminales. ---
  const brandPanel = (
    <aside className={styles.brand}>
      <div className={styles.brandTop}>
        <img src="/logos/Horizontal_Blanco.png" alt="Obertrack" className={styles.logo} />
      </div>

      <div className={styles.brandBody}>
        <p className={styles.brandKicker}>Bienvenido a Obertrack</p>
        <h1 className={styles.brandName}>{landing?.professional_name ?? 'Tu inducción'}</h1>
        <p className={styles.brandText}>
          Completa esta breve inducción para activar tu acceso a la plataforma.
        </p>

        {landing && !result && (currentStep === 'video' || currentStep === 'quiz') && (
          <ol className={styles.stepper} aria-label="Progreso de la inducción">
            {stepList.map((s, i) => {
              const activeIdx = stepList.findIndex((x) => x.key === currentStep)
              const state = i < activeIdx ? 'done' : i === activeIdx ? 'active' : 'todo'
              return (
                <li key={s.key} className={styles[`step_${state}`]}>
                  <span className={styles.stepDot}>{state === 'done' ? '✓' : i + 1}</span>
                  <span className={styles.stepLabel}>{s.label}</span>
                </li>
              )
            })}
          </ol>
        )}
      </div>

      <p className={styles.brandFoot}>
        ¿Problemas con tu inducción? Responde al correo que recibiste y te ayudamos.
      </p>
    </aside>
  )

  // --- Contenido del panel derecho según el estado ---
  let content: React.ReactNode

  if (loading) {
    content = (
      <div className={styles.state}>
        <div className={styles.spinner} aria-hidden />
        <p className={styles.stateText}>Cargando tu inducción...</p>
      </div>
    )
  } else if (loadError || !landing) {
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconNeutral}`}>
          <AlertTriangle size={30} />
        </div>
        <h2 className={styles.stateTitle}>Enlace no válido</h2>
        <p className={styles.stateText}>{loadError}</p>
      </div>
    )
  } else if (!result && landing.status === 'passed') {
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconOk}`}>
          <CheckCircle2 size={30} />
        </div>
        <h2 className={styles.stateTitle}>Ya completaste tu inducción</h2>
        <p className={styles.stateText}>
          Tu acceso está habilitado. Revisa tu correo para crear tu contraseña.
        </p>
        <a className={styles.primaryBtn} href="/login">
          Ir a Obertrack <ArrowRight size={18} />
        </a>
      </div>
    )
  } else if (!result && landing.status === 'blocked') {
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconWarn}`}>
          <LifeBuoy size={30} />
        </div>
        <h2 className={styles.stateTitle}>Tu acceso está en revisión</h2>
        <p className={styles.stateText}>
          Agotaste tus intentos de inducción. Nuestro equipo de soporte se pondrá en contacto
          contigo para acompañarte.
        </p>
      </div>
    )
  } else if (currentStep === 'video') {
    content = (
      <section className={styles.pane}>
        <div className={styles.paneHead}>
          <h2 className={styles.paneTitle}>{landing.video_title || 'Video de inducción'}</h2>
          {landing.video_duration_min ? (
            <span className={styles.badge}>
              <Clock size={14} /> {landing.video_duration_min} min
            </span>
          ) : null}
        </div>

        {embedUrl ? (
          <div className={styles.videoWrap}>
            <iframe
              src={embedUrl}
              title={landing.video_title || 'Video de inducción'}
              allow="autoplay; encrypted-media"
              allowFullScreen
            />
          </div>
        ) : (
          <p className={styles.note}>
            El video no se puede reproducir aquí.{' '}
            <a href={landing.video_url} target="_blank" rel="noreferrer">
              Ábrelo en una pestaña nueva
            </a>
            .
          </p>
        )}

        <div className={styles.actions}>
          <button type="button" className={styles.primaryBtn} onClick={() => setStep('quiz')}>
            <PlayCircle size={18} /> Ya vi el video, continuar
          </button>
        </div>
      </section>
    )
  } else if (currentStep === 'quiz') {
    content = (
      <section className={styles.pane}>
        <div className={styles.paneHead}>
          <h2 className={styles.paneTitle}>{landing.survey_title || 'Cuestionario'}</h2>
        </div>
        {landing.description && <p className={styles.note}>{landing.description}</p>}

        <div className={styles.meta}>
          <span className={styles.metaItem}>
            <span className={styles.metaLabel}>Mínimo</span>
            <strong>{landing.passing_score}%</strong>
          </span>
          <span className={styles.metaDivider} />
          <span className={styles.metaItem}>
            <span className={styles.metaLabel}>Intentos</span>
            <strong>
              {landing.attempts_left} de {landing.max_attempts}
            </strong>
          </span>
        </div>

        {submitError && <div className={styles.error}>{submitError}</div>}

        <form onSubmit={handleSubmit} className={styles.form}>
          {landing.questions.map((q, index) => (
            <div key={q.id} className={styles.question}>
              <label className={styles.questionLabel}>
                <span className={styles.questionNumber}>{index + 1}</span>
                <span>
                  {q.text}
                  {q.is_required && <span className={styles.required}>*</span>}
                </span>
              </label>

              {q.type === 'text' && (
                <textarea
                  className={styles.textInput}
                  required={q.is_required}
                  value={answers[q.id] ?? ''}
                  onChange={(e) => setAnswer(q.id, e.target.value)}
                  placeholder="Escribe tu respuesta..."
                />
              )}

              {q.type === 'choice' && (
                <div className={styles.options}>
                  {q.options.map((opt, i) => (
                    <label
                      key={i}
                      className={`${styles.option} ${answers[q.id] === opt ? styles.optionSel : ''}`}
                    >
                      <input
                        type="radio"
                        name={`q_${q.id}`}
                        required={q.is_required}
                        checked={answers[q.id] === opt}
                        onChange={() => setAnswer(q.id, opt)}
                      />
                      <span className={styles.radio} aria-hidden />
                      <span>{opt}</span>
                    </label>
                  ))}
                </div>
              )}

              {q.type === 'rating' && (
                <div className={styles.ratingRow}>
                  {[1, 2, 3, 4, 5].map((n) => (
                    <label
                      key={n}
                      className={`${styles.rating} ${answers[q.id] === String(n) ? styles.ratingSel : ''}`}
                    >
                      <input
                        type="radio"
                        name={`q_${q.id}`}
                        required={q.is_required}
                        checked={answers[q.id] === String(n)}
                        onChange={() => setAnswer(q.id, String(n))}
                      />
                      {n}
                    </label>
                  ))}
                </div>
              )}
            </div>
          ))}

          <div className={styles.actions}>
            {landing.video_url && (
              <button type="button" className={styles.secondaryBtn} onClick={() => setStep('video')}>
                Volver al video
              </button>
            )}
            <button type="submit" className={styles.primaryBtn} disabled={submitting}>
              {submitting ? 'Enviando...' : 'Enviar respuestas'}
            </button>
          </div>
        </form>
      </section>
    )
  } else if (result) {
    const tone = result.passed ? 'ok' : 'warn'
    content = (
      <section className={styles.result}>
        <div className={`${styles.stateIcon} ${result.passed ? styles.iconOk : styles.iconWarn}`}>
          {result.passed ? (
            <CheckCircle2 size={30} />
          ) : result.status === 'blocked' ? (
            <LifeBuoy size={30} />
          ) : (
            <AlertTriangle size={30} />
          )}
        </div>

        {/* Anillo de puntaje */}
        <div
          className={`${styles.scoreRing} ${styles[`ring_${tone}`]}`}
          style={{ ['--pct' as string]: `${Math.min(100, Math.round(result.score))}` }}
        >
          <div className={styles.scoreInner}>
            <span className={styles.scoreNum}>{Math.round(result.score)}%</span>
            <span className={styles.scoreMin}>mín. {result.passing_score}%</span>
          </div>
        </div>

        <h2 className={styles.stateTitle}>
          {result.passed
            ? '¡Aprobaste tu inducción!'
            : result.status === 'blocked'
              ? 'No alcanzaste el mínimo'
              : 'Casi lo logras'}
        </h2>
        <p className={styles.stateText}>{result.message}</p>

        {result.passed && (
          <a className={styles.primaryBtn} href="/login">
            Ir a Obertrack <ArrowRight size={18} />
          </a>
        )}

        {!result.passed && result.status === 'pending' && (
          <button type="button" className={styles.primaryBtn} onClick={handleRetry}>
            <RotateCcw size={18} /> Intentar de nuevo
          </button>
        )}
      </section>
    )
  }

  return (
    <div className={styles.page}>
      <div className={styles.shell}>
        {brandPanel}
        <main className={styles.content}>{content}</main>
      </div>
    </div>
  )
}
