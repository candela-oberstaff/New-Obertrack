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
import { BadgeMedallion } from '../components/Badges/BadgeMedallion'
import { FileCheck } from 'lucide-react'
import badgeStyles from '../components/Badges/Badges.module.css'
import styles from './Induction.module.css'

/** Paso visible dentro del bloque actual. */
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
 * contratado desde Obersuite: recorre los bloques de su programa en orden
 * (cada uno con su video y su cuestionario) y, si aprueba todos, se le
 * habilita el acceso a Obertrack.
 *
 * No requiere sesión: el token del enlace es la credencial.
 *
 * Layout: wizard de dos paneles. A la izquierda, panel de marca (logo,
 * bienvenida, progreso por bloque); a la derecha, el contenido del paso actual
 * del bloque en curso. El servidor decide cuál es el bloque actual: aquí solo
 * se pinta y se envía lo que él diga.
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
      // Si el bloque actual no trae video, es solo cuestionario.
      setStep(data.current?.video_url ? 'video' : 'quiz')
    } catch (err) {
      setLoadError(errorMessage(err, 'No pudimos cargar tu inducción. Verifica el enlace de tu correo.'))
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    void load()
  }, [load])

  const current = landing?.current ?? null

  const embedUrl = useMemo(
    () => (current?.video_url ? buildEmbedUrl(current.video_url) : null),
    [current?.video_url]
  )

  const currentStep: Step = result ? 'result' : step

  const setAnswer = (questionId: number, value: string) =>
    setAnswers((prev) => ({ ...prev, [questionId]: value }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!current) return

    setSubmitting(true)
    setSubmitError('')
    try {
      const payload: InductionAnswer[] = current.questions.map((q) => ({
        question_id: q.id,
        value: answers[q.id] ?? '',
      }))
      const res = await inductionService.submit(token, current.block_id, payload)
      setResult(res)
      setStep('result')
    } catch (err) {
      setSubmitError(errorMessage(err, 'No pudimos enviar tus respuestas. Intenta de nuevo.'))
    } finally {
      setSubmitting(false)
    }
  }

  // Reintento o paso al siguiente bloque: limpia las respuestas y vuelve a
  // pedir la landing, que trae el bloque que toca con sus intentos al día.
  const handleContinue = async () => {
    setAnswers({})
    setResult(null)
    setSubmitError('')
    await load()
  }

  // --- Panel de marca (izquierda): un paso por bloque. Bajo el bloque en
  // curso se marca si va por el video o por el cuestionario. ---
  const showStepper =
    !!landing && landing.status === 'pending' && !!current && (currentStep === 'video' || currentStep === 'quiz')

  const brandPanel = (
    <aside className={styles.brand}>
      <div className={styles.brandTop}>
        <img src="/logos/Horizontal_Blanco.png" alt="Obertrack" className={styles.logo} />
      </div>

      <div className={styles.brandBody}>
        <p className={styles.brandKicker}>Bienvenido a Obertrack</p>
        <h1 className={styles.brandName}>{landing?.professional_name ?? 'Tu inducción'}</h1>
        <p className={styles.brandText}>
          {landing && !landing.gates_access
            ? `Completa ${landing.total_blocks > 1 ? `los ${landing.total_blocks} bloques de` : ''} esta capacitación para ganar tus insignias. Tu acceso no cambia.`
            : landing && landing.total_blocks > 1
              ? `Completa los ${landing.total_blocks} bloques de tu inducción para activar tu acceso a la plataforma.`
              : 'Completa esta breve inducción para activar tu acceso a la plataforma.'}
        </p>

        {showStepper && landing && current && (
          <ol className={styles.stepper} aria-label="Progreso de la inducción">
            {landing.blocks.map((b, i) => {
              const isCurrent = b.block_id === current.block_id
              const state = b.status === 'passed' ? 'done' : isCurrent ? 'active' : 'todo'
              return (
                <li key={b.block_id} className={styles[`step_${state}`]}>
                  <span className={styles.stepDot}>{state === 'done' ? '✓' : i + 1}</span>
                  <span className={styles.stepLabel}>
                    {b.name}
                    {isCurrent && (
                      <span className={styles.stepSub}>
                        {b.has_video && (
                          <span className={currentStep === 'video' ? styles.stepSubActive : ''}>Video</span>
                        )}
                        {b.has_video && <span className={styles.stepSubSep}>·</span>}
                        <span className={currentStep === 'quiz' ? styles.stepSubActive : ''}>Cuestionario</span>
                      </span>
                    )}
                  </span>
                </li>
              )
            })}
          </ol>
        )}
      </div>

      {landing && landing.badges && landing.badges.length > 0 && (
        <div className={styles.brandBadges}>
          <span className={styles.brandBadgesLabel}>Tus insignias</span>
          <div className={styles.brandBadgesRow}>
            {landing.badges.map((b) => (
              <BadgeMedallion key={b.id} icon={b.icon} color={b.color} size="sm" title={b.title} />
            ))}
          </div>
        </div>
      )}

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
        <h2 className={styles.stateTitle}>
          {landing.gates_access ? 'Ya completaste tu inducción' : 'Ya completaste esta capacitación'}
        </h2>
        <p className={styles.stateText}>
          {landing.gates_access
            ? 'Tu acceso está habilitado. Revisa tu correo para crear tu contraseña.'
            : 'Tus insignias ya están en tu perfil.'}
        </p>
        {landing.certificate && (
          <a className={styles.secondaryBtn} href={landing.certificate.download_url}>
            <FileCheck size={18} /> Descargar certificado
          </a>
        )}
        <a className={styles.primaryBtn} href={landing.gates_access ? '/login' : '/profile'}>
          {landing.gates_access ? 'Ir a Obertrack' : 'Volver a Obertrack'} <ArrowRight size={18} />
        </a>
      </div>
    )
  } else if (!result && landing.status === 'blocked') {
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconWarn}`}>
          <LifeBuoy size={30} />
        </div>
        <h2 className={styles.stateTitle}>
          {landing.gates_access ? 'Tu acceso está en revisión' : 'No aprobaste esta capacitación'}
        </h2>
        <p className={styles.stateText}>
          {landing.gates_access
            ? 'Agotaste tus intentos de inducción. Nuestro equipo de soporte se pondrá en contacto contigo para acompañarte.'
            : 'Agotaste tus intentos. Tu acceso a Obertrack no cambia; nuestro equipo se pondrá en contacto contigo.'}
        </p>
        {!landing.gates_access && (
          <a className={styles.primaryBtn} href="/profile">
            Volver a Obertrack <ArrowRight size={18} />
          </a>
        )}
      </div>
    )
  } else if (!result && !current) {
    // Pendiente pero sin bloque que mostrar: programa vacío o dato corrupto.
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconNeutral}`}>
          <AlertTriangle size={30} />
        </div>
        <h2 className={styles.stateTitle}>Tu inducción no está lista</h2>
        <p className={styles.stateText}>
          Todavía no hay contenido asignado. Responde al correo que recibiste y te ayudamos.
        </p>
      </div>
    )
  } else if (currentStep === 'video' && current) {
    content = (
      <section className={styles.pane}>
        <p className={styles.blockKicker}>
          Bloque {current.order_index + 1} de {landing.total_blocks} · {current.name}
        </p>
        <div className={styles.paneHead}>
          <h2 className={styles.paneTitle}>{current.video_title || 'Video de inducción'}</h2>
          {current.video_duration_min ? (
            <span className={styles.badge}>
              <Clock size={14} /> {current.video_duration_min} min
            </span>
          ) : null}
        </div>

        {embedUrl ? (
          <div className={styles.videoWrap}>
            <iframe
              src={embedUrl}
              title={current.video_title || 'Video de inducción'}
              allow="autoplay; encrypted-media"
              allowFullScreen
            />
          </div>
        ) : (
          <p className={styles.note}>
            El video no se puede reproducir aquí.{' '}
            <a href={current.video_url} target="_blank" rel="noreferrer">
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
  } else if (currentStep === 'quiz' && current) {
    content = (
      <section className={styles.pane}>
        <p className={styles.blockKicker}>
          Bloque {current.order_index + 1} de {landing.total_blocks} · {current.name}
        </p>
        <div className={styles.paneHead}>
          <h2 className={styles.paneTitle}>{current.survey_title || 'Cuestionario'}</h2>
        </div>
        {current.description && <p className={styles.note}>{current.description}</p>}

        <div className={styles.meta}>
          <span className={styles.metaItem}>
            <span className={styles.metaLabel}>Mínimo</span>
            <strong>{current.passing_score}%</strong>
          </span>
          <span className={styles.metaDivider} />
          <span className={styles.metaItem}>
            <span className={styles.metaLabel}>Intentos</span>
            <strong>
              {current.attempts_left} de {current.max_attempts}
            </strong>
          </span>
        </div>

        {submitError && <div className={styles.error}>{submitError}</div>}

        <form onSubmit={handleSubmit} className={styles.form}>
          {current.questions.map((q, index) => (
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
            {current.video_url && (
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
    const title = result.completed
      ? '¡Aprobaste tu inducción!'
      : result.passed
        ? `¡Aprobaste el bloque ${result.block_index} de ${landing.total_blocks}!`
        : result.status === 'blocked'
          ? 'No alcanzaste el mínimo'
          : 'Casi lo logras'
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

        <h2 className={styles.stateTitle}>{title}</h2>
        <p className={styles.stateText}>{result.message}</p>

        {result.badges_earned && result.badges_earned.length > 0 && (
          <div className={badgeStyles.reveal} aria-live="polite">
            {result.badges_earned.map((b) => (
              <div key={b.id} className={badgeStyles.revealItem}>
                <span className={badgeStyles.revealKicker}>
                  {b.kind === 'merit' ? 'Mérito' : 'Insignia desbloqueada'}
                </span>
                <BadgeMedallion icon={b.icon} color={b.color} size="lg" shine title={b.title} />
                <span className={badgeStyles.revealTitle}>{b.title}</span>
              </div>
            ))}
          </div>
        )}

        {result.completed && result.certificate && (
          <a className={styles.secondaryBtn} href={result.certificate.download_url}>
            <FileCheck size={18} /> Descargar certificado
          </a>
        )}

        {result.completed && (
          <a className={styles.primaryBtn} href={landing.gates_access ? '/login' : '/profile'}>
            {landing.gates_access ? 'Ir a Obertrack' : 'Ver mis insignias'} <ArrowRight size={18} />
          </a>
        )}

        {result.passed && !result.completed && (
          <button type="button" className={styles.primaryBtn} onClick={handleContinue}>
            Continuar con: {result.next_block_name || 'siguiente bloque'} <ArrowRight size={18} />
          </button>
        )}

        {!result.passed && result.status === 'pending' && (
          <button type="button" className={styles.primaryBtn} onClick={handleContinue}>
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
