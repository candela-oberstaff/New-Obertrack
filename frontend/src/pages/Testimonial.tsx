import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  ChevronDown,
  Clock,
  Heart,
  Star,
} from 'lucide-react'

import SignaturePad, { type SignatureMode } from '../components/Testimonials/SignaturePad'
import {
  testimonialService,
  type TestimonialAnswer,
  type TestimonialLanding,
} from '../services/testimonial.service'
import styles from './Testimonial.module.css'

/** Paso visible de la página. */
type Step = 'write' | 'sign' | 'done'

function errorMessage(err: unknown, fallback: string): string {
  const responseError =
    err && typeof err === 'object' && 'response' in err
      ? (err as { response?: { data?: { error?: string } } }).response?.data?.error
      : undefined
  return responseError || fallback
}

/**
 * Página pública del testimonio. La abre quien recibió la solicitud por correo:
 * escribe su experiencia, elige qué autoriza a publicar y firma.
 *
 * No requiere sesión — el token del enlace es la credencial —, así que replica
 * el wizard de dos paneles de la inducción en lugar de vivir dentro del Layout.
 */
export default function Testimonial() {
  const { token = '' } = useParams<{ token: string }>()

  const [landing, setLanding] = useState<TestimonialLanding | null>(null)
  const [loadError, setLoadError] = useState('')
  const [loading, setLoading] = useState(true)

  const [step, setStep] = useState<Step>('write')
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  // --- Respuesta ---
  const [rating, setRating] = useState(0)
  const [quote, setQuote] = useState('')
  const [promptAnswers, setPromptAnswers] = useState<Record<number, string>>({})
  const [showPrompts, setShowPrompts] = useState(false)

  // --- Autorización ---
  const [allowPublicName, setAllowPublicName] = useState(true)
  const [allowRole, setAllowRole] = useState(true)
  const [allowPhoto, setAllowPhoto] = useState(false)
  const [allowLogo, setAllowLogo] = useState(false)
  const [consentAccepted, setConsentAccepted] = useState(false)
  const [signatureName, setSignatureName] = useState('')
  const [signatureImage, setSignatureImage] = useState('')
  const [signatureMode, setSignatureMode] = useState<SignatureMode>('drawn')

  const load = useCallback(async () => {
    setLoading(true)
    setLoadError('')
    try {
      const data = await testimonialService.getLanding(token)
      setLanding(data)
      // El nombre viene precargado, pero es editable: quien firma puede tener
      // su nombre legal escrito de otra forma que en su perfil.
      setSignatureName(data.previous?.signature_name || data.recipient_name)

      // Corrección: se repuebla todo lo que ya había escrito. Solo la firma se
      // deja en blanco a propósito — hay que volver a trazarla, porque la
      // anterior autorizaba un texto que puede haber cambiado.
      if (data.previous) {
        const p = data.previous
        setRating(p.rating)
        setQuote(p.quote)
        setAllowPublicName(p.allow_public_name)
        setAllowRole(p.allow_role)
        setAllowPhoto(p.allow_photo)
        setAllowLogo(p.allow_logo)
        const byPrompt = new Map(p.answers.map((a) => [a.prompt, a.answer]))
        const restored: Record<number, string> = {}
        data.prompts.forEach((prompt, i) => {
          const previousAnswer = byPrompt.get(prompt)
          if (previousAnswer) restored[i] = previousAnswer
        })
        setPromptAnswers(restored)
        if (Object.keys(restored).length > 0) setShowPrompts(true)
      }
    } catch (err) {
      setLoadError(
        errorMessage(err, 'No pudimos cargar esta solicitud. Verifica el enlace de tu correo.')
      )
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    void load()
  }, [load])

  const isCompany = landing?.audience === 'company'

  const stepList = useMemo(
    () => [
      { key: 'write' as Step, label: 'Tu experiencia' },
      { key: 'sign' as Step, label: 'Autorización' },
      { key: 'done' as Step, label: 'Listo' },
    ],
    []
  )

  // Basta con que haya texto: no hay largo mínimo, a propósito. Un testimonio
  // sincero puede ser una sola frase, y exigir una cuenta de caracteres empuja a
  // rellenar, que es lo que hace sonar falso un testimonio.
  const quoteLength = quote.trim().length
  const quoteReady = quoteLength > 0
  const signReady = consentAccepted && signatureName.trim().length >= 3 && signatureImage !== ''

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!landing || !signReady) return

    setSubmitting(true)
    setSubmitError('')
    try {
      // Solo viajan las preguntas que sí se respondieron: las vacías no aportan
      // nada al material y ensucian la constancia.
      const answers: TestimonialAnswer[] = landing.prompts
        .map((prompt, i) => ({ prompt, answer: (promptAnswers[i] ?? '').trim() }))
        .filter((a) => a.answer !== '')

      await testimonialService.submit(token, {
        rating,
        quote: quote.trim(),
        answers,
        allow_public_name: allowPublicName,
        allow_role: allowRole,
        allow_photo: allowPhoto,
        allow_logo: allowLogo,
        consent_accepted: consentAccepted,
        signature_name: signatureName.trim(),
        signature_image: signatureImage,
        signature_mode: signatureMode,
      })
      setStep('done')
    } catch (err) {
      setSubmitError(errorMessage(err, 'No pudimos enviar tu testimonio. Intenta de nuevo.'))
    } finally {
      setSubmitting(false)
    }
  }

  // --- Panel de marca (izquierda) ---
  const brandPanel = (
    <aside className={styles.brand}>
      <div className={styles.brandTop}>
        <img src="/logos/Horizontal_Blanco.png" alt="Obertrack" className={styles.logo} />
      </div>

      <div className={styles.brandBody}>
        <p className={styles.brandKicker}>Testimonio</p>
        <h1 className={styles.brandName}>{landing?.recipient_name ?? 'Tu experiencia'}</h1>
        {(landing?.recipient_role || landing?.recipient_company) && (
          <p className={styles.brandRole}>
            {[landing?.recipient_role, landing?.recipient_company].filter(Boolean).join(' · ')}
          </p>
        )}
        <p className={styles.brandText}>
          Gracias por tomarte unos minutos. Lo que escribas aquí nos ayuda a que más gente nos
          conozca.
        </p>

        {landing && step !== 'done' && (
          <ol className={styles.stepper} aria-label="Progreso del testimonio">
            {stepList.map((s, i) => {
              const activeIdx = stepList.findIndex((x) => x.key === step)
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
        ¿Prefieres no participar? Puedes cerrar esta página sin enviar nada.
      </p>
    </aside>
  )

  // --- Contenido del panel derecho ---
  let content: React.ReactNode

  if (loading) {
    content = (
      <div className={styles.state}>
        <div className={styles.spinner} aria-hidden />
        <p className={styles.stateText}>Cargando tu solicitud...</p>
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
  } else if (step !== 'done' && landing.status !== 'pending' && landing.status !== 'changes_requested') {
    // Ya respondido: no es un error, es un agradecimiento.
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconOk}`}>
          <CheckCircle2 size={30} />
        </div>
        <h2 className={styles.stateTitle}>Ya nos enviaste tu testimonio</h2>
        <p className={styles.stateText}>
          Lo recibimos y lo tenemos guardado. Gracias por tomarte el tiempo.
        </p>
      </div>
    )
  } else if (step !== 'done' && landing.expired) {
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconWarn}`}>
          <Clock size={30} />
        </div>
        <h2 className={styles.stateTitle}>El enlace venció</h2>
        <p className={styles.stateText}>
          Este enlace ya no está activo. Responde al correo que recibiste y te enviamos uno nuevo.
        </p>
      </div>
    )
  } else if (step === 'write') {
    content = (
      <section className={styles.pane}>
        <div className={styles.paneHead}>
          <h2 className={styles.paneTitle}>{landing.headline}</h2>
        </div>
        {/* Lo primero que tiene que ver quien vuelve a corregir: qué hay que
            arreglar. Va antes de la nota de presentación, que ya leyó. */}
        {landing.change_reason && (
          <div className={styles.changeBox}>
            <p className={styles.changeTitle}>
              <AlertTriangle size={15} /> Un detalle por corregir
            </p>
            <p className={styles.changeReason}>{landing.change_reason}</p>
            <p className={styles.changeHint}>
              Dejamos lo que ya habías escrito. Ajusta lo necesario y vuelve a firmar.
            </p>
          </div>
        )}
        {!landing.change_reason && <p className={styles.note}>{landing.intro_message}</p>}

        {/* Calificación. Opcional a propósito: obligarla sesga la respuesta. */}
        <div className={styles.field}>
          <label className={styles.fieldLabel}>
            ¿Cómo calificarías tu experiencia?
            <span className={styles.optional}>Opcional</span>
          </label>
          <div className={styles.stars}>
            {[1, 2, 3, 4, 5].map((n) => (
              <button
                key={n}
                type="button"
                className={`${styles.star} ${n <= rating ? styles.starOn : ''}`}
                onClick={() => setRating(n === rating ? 0 : n)}
                aria-label={`${n} de 5`}
                aria-pressed={n <= rating}
              >
                <Star size={26} fill={n <= rating ? 'currentColor' : 'none'} />
              </button>
            ))}
          </div>
        </div>

        {/* Testimonio. */}
        <div className={styles.field}>
          <label className={styles.fieldLabel} htmlFor="quote">
            Tu testimonio
            <span className={styles.required}>*</span>
          </label>
          <textarea
            id="quote"
            className={styles.textArea}
            value={quote}
            onChange={(e) => setQuote(e.target.value)}
            placeholder="Escribe con tus palabras cómo ha sido tu experiencia..."
            rows={7}
          />
          {/* La cuenta es informativa: acompaña, no exige. Con el campo vacío no
              se muestra nada para no recibir a nadie con un "0". */}
          {quoteLength > 0 && (
            <div className={styles.counterRow}>
              <span className={styles.counter}>{quoteLength} caracteres</span>
            </div>
          )}
        </div>

        {/* Preguntas guía: plegadas por defecto para no abrumar la pantalla. */}
        {landing.prompts.length > 0 && (
          <div className={styles.promptsBox}>
            <button
              type="button"
              className={styles.promptsToggle}
              onClick={() => setShowPrompts((v) => !v)}
              aria-expanded={showPrompts}
            >
              <ChevronDown
                size={16}
                className={showPrompts ? styles.chevronOpen : styles.chevron}
              />
              ¿No sabes por dónde empezar? Responde estas preguntas
            </button>

            {showPrompts && (
              <div className={styles.promptsBody}>
                <p className={styles.promptsHint}>
                  Son opcionales y nos sirven de referencia. Lo que se publica es el texto de
                  arriba.
                </p>
                {landing.prompts.map((prompt, i) => (
                  <div key={i} className={styles.prompt}>
                    <label className={styles.promptLabel} htmlFor={`prompt_${i}`}>
                      {prompt}
                    </label>
                    <textarea
                      id={`prompt_${i}`}
                      className={styles.textArea}
                      rows={3}
                      value={promptAnswers[i] ?? ''}
                      onChange={(e) =>
                        setPromptAnswers((prev) => ({ ...prev, [i]: e.target.value }))
                      }
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        <div className={styles.actions}>
          <button
            type="button"
            className={styles.primaryBtn}
            disabled={!quoteReady}
            onClick={() => setStep('sign')}
          >
            Continuar <ArrowRight size={18} />
          </button>
        </div>
      </section>
    )
  } else if (step === 'sign') {
    content = (
      <section className={styles.pane}>
        <div className={styles.paneHead}>
          <h2 className={styles.paneTitle}>Autorización y firma</h2>
        </div>
        <p className={styles.note}>
          Para poder usar tu testimonio necesitamos tu permiso por escrito. Lee el texto, elige qué
          se puede mostrar y firma abajo.
        </p>

        {submitError && <div className={styles.error}>{submitError}</div>}

        <form onSubmit={handleSubmit} className={styles.form}>
          {/* Texto legal. Se muestra completo, sin recortes ni enlaces: nadie
              debería firmar algo que tenga que abrir en otra parte. */}
          <div className={styles.consentBox}>
            <p className={styles.consentText}>{landing.consent_text}</p>
            <p className={styles.consentVersion}>Redacción {landing.consent_version}</p>
          </div>

          {/* Alcance del permiso. */}
          <div className={styles.field}>
            <span className={styles.fieldLabel}>¿Qué podemos mostrar junto a tu testimonio?</span>
            <div className={styles.checkList}>
              <label className={styles.check}>
                <input
                  type="checkbox"
                  checked={allowPublicName}
                  onChange={(e) => setAllowPublicName(e.target.checked)}
                />
                <span>Mi nombre</span>
              </label>
              <label className={styles.check}>
                <input
                  type="checkbox"
                  checked={allowRole}
                  onChange={(e) => setAllowRole(e.target.checked)}
                />
                <span>Mi cargo y la empresa</span>
              </label>
              {isCompany ? (
                <label className={styles.check}>
                  <input
                    type="checkbox"
                    checked={allowLogo}
                    onChange={(e) => setAllowLogo(e.target.checked)}
                  />
                  <span>El logo de la empresa</span>
                </label>
              ) : (
                <label className={styles.check}>
                  <input
                    type="checkbox"
                    checked={allowPhoto}
                    onChange={(e) => setAllowPhoto(e.target.checked)}
                  />
                  <span>Mi fotografía</span>
                </label>
              )}
            </div>
          </div>

          {/* Consentimiento explícito. */}
          <label className={`${styles.check} ${styles.consentCheck}`}>
            <input
              type="checkbox"
              checked={consentAccepted}
              onChange={(e) => setConsentAccepted(e.target.checked)}
            />
            <span>He leído y acepto la autorización de arriba.</span>
          </label>

          {/* Firma. */}
          <div className={styles.field}>
            <label className={styles.fieldLabel} htmlFor="signatureName">
              Nombre completo
              <span className={styles.required}>*</span>
            </label>
            <input
              id="signatureName"
              className={styles.textInput}
              value={signatureName}
              onChange={(e) => setSignatureName(e.target.value)}
              placeholder="Tal como aparece en tu documento"
              autoComplete="name"
            />
          </div>

          <div className={styles.field}>
            <span className={styles.fieldLabel}>
              Tu firma
              <span className={styles.required}>*</span>
            </span>
            <SignaturePad
              onChange={(img, mode) => {
                setSignatureImage(img)
                setSignatureMode(mode)
              }}
              suggestedName={signatureName}
              disabled={submitting}
              hint={
                landing.change_reason
                  ? 'Vuelve a firmar: tu firma anterior autorizaba el texto sin corregir.'
                  : undefined
              }
            />
          </div>

          <div className={styles.actions}>
            <button
              type="button"
              className={styles.secondaryBtn}
              onClick={() => setStep('write')}
              disabled={submitting}
            >
              <ArrowLeft size={16} /> Volver
            </button>
            <button type="submit" className={styles.primaryBtn} disabled={!signReady || submitting}>
              {submitting ? 'Enviando...' : 'Firmar y enviar'}
            </button>
          </div>
        </form>
      </section>
    )
  } else {
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconOk}`}>
          <Heart size={30} />
        </div>
        <h2 className={styles.stateTitle}>¡Gracias!</h2>
        <p className={styles.stateText}>
          Recibimos tu testimonio y tu autorización. Significa mucho para nosotros que te hayas
          tomado el tiempo.
        </p>
      </div>
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
