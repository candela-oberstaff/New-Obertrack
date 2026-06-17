import { useEffect, useState } from 'react'
import { FileText, PenLine, ChevronLeft, Send, Mail, Loader2, Inbox } from 'lucide-react'
import { Modal, Button } from '../ui'
import { emailService, type EmailTemplate } from '../../services/emailService'
import { adminService } from '../../services/api'
import { useNotification } from '../../context/NotificationContext'

export interface ComposerRecipient {
  id: number
  name: string
  email: string
}

interface EmailComposerModalProps {
  isOpen: boolean
  onClose: () => void
  recipient: ComposerRecipient | null
  /** Asunto sugerido al redactar una nueva (editable). */
  defaultSubject?: string
  /** Cuerpo sugerido al redactar una nueva (editable). */
  defaultBody?: string
  /**
   * Registra el contacto vía adminService.logContact(recipient.id, 'email').
   * Solo válido cuando recipient.id es un id de usuario (ej. panel de profesionales).
   * Para destinatarios que no son usuarios (ej. responsables de empresa) pásalo como false.
   */
  logContact?: boolean
}

type Step = 'choose' | 'pick-template' | 'compose'

/** Convierte el contenido (JSON de bloques) de una plantilla de Tools en texto editable. */
function templateToText(content: string): string {
  try {
    const blocks = JSON.parse(content)
    if (!Array.isArray(blocks)) return ''
    return blocks
      .map((b: { type?: string; content?: string }) => {
        if (!b || typeof b.content !== 'string') return ''
        switch (b.type) {
          case 'text':
          case 'button':
            // Los bloques de texto pueden traer HTML inline; lo quitamos.
            return b.content.replace(/<br\s*\/?>(?=)/gi, '\n').replace(/<[^>]+>/g, '').trim()
          case 'image':
            return b.content ? `[Imagen: ${b.content}]` : ''
          default:
            return ''
        }
      })
      .filter(Boolean)
      .join('\n\n')
  } catch {
    return ''
  }
}

/**
 * Convierte el texto plano del cuerpo en párrafos HTML. No añade banner ni footer:
 * el backend (BrevoService.SendEmail) ya envuelve el contenido con la cabecera y el
 * pie de marca de Obertrack, así que solo enviamos el contenido interno.
 */
function bodyToHTML(body: string): string {
  const escaped = body
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  return escaped
    .split(/\n{2,}/)
    .map(p => `<p style="margin:0 0 16px;">${p.replace(/\n/g, '<br/>')}</p>`)
    .join('\n')
}

export function EmailComposerModal({
  isOpen,
  onClose,
  recipient,
  defaultSubject = 'Seguimiento de actividad en Obertrack',
  defaultBody = '',
  logContact = true,
}: EmailComposerModalProps) {
  const notify = useNotification()
  const [step, setStep] = useState<Step>('choose')
  const [templates, setTemplates] = useState<EmailTemplate[]>([])
  const [loadingTemplates, setLoadingTemplates] = useState(false)
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const [sending, setSending] = useState(false)

  // Reinicia el flujo cada vez que se abre el modal.
  useEffect(() => {
    if (isOpen) {
      setStep('choose')
      setSubject('')
      setBody('')
      setSending(false)
    }
  }, [isOpen, recipient?.id])

  const loadTemplates = async () => {
    setStep('pick-template')
    setLoadingTemplates(true)
    try {
      const res = await emailService.getTemplates()
      setTemplates(res.filter(t => t.is_active !== false))
    } catch (err) {
      console.error('Error cargando plantillas:', err)
      notify.error('No se pudieron cargar las plantillas.')
      setTemplates([])
    } finally {
      setLoadingTemplates(false)
    }
  }

  const chooseNew = () => {
    setSubject(defaultSubject)
    setBody(defaultBody)
    setStep('compose')
  }

  const pickTemplate = (t: EmailTemplate) => {
    setSubject(t.subject || defaultSubject)
    setBody(templateToText(t.content))
    setStep('compose')
  }

  const handleSend = async () => {
    if (!recipient) return
    if (!subject.trim() || !body.trim()) {
      notify.warning('Completa el asunto y el cuerpo del correo.')
      return
    }
    setSending(true)
    try {
      await emailService.sendQuickEmail({
        to_email: recipient.email,
        to_name: recipient.name,
        subject: subject.trim(),
        html_content: bodyToHTML(body.trim()),
      })
      if (logContact) adminService.logContact(recipient.id, 'email').catch(() => {})
      notify.success(`Correo enviado a ${recipient.name}.`)
      onClose()
    } catch (err: any) {
      console.error('Error enviando correo:', err)
      const serverMsg = err?.response?.data?.error
      notify.error(serverMsg ? `No se pudo enviar: ${serverMsg}` : 'No se pudo enviar el correo. Inténtalo de nuevo.')
    } finally {
      setSending(false)
    }
  }

  if (!recipient) return null

  const title =
    step === 'choose'
      ? `Enviar correo a ${recipient.name}`
      : step === 'pick-template'
      ? 'Elegir plantilla'
      : 'Redactar correo'

  const footer =
    step === 'compose' ? (
      <>
        <Button variant="secondary" leftIcon={<ChevronLeft size={15} />} onClick={() => setStep('choose')} disabled={sending}>
          Volver
        </Button>
        <Button loading={sending} leftIcon={<Send size={15} />} onClick={handleSend}>
          {sending ? 'Enviando…' : 'Enviar'}
        </Button>
      </>
    ) : step === 'pick-template' ? (
      <Button variant="secondary" leftIcon={<ChevronLeft size={15} />} onClick={() => setStep('choose')}>
        Volver
      </Button>
    ) : undefined

  const labelStyle: React.CSSProperties = {
    fontSize: 11,
    fontWeight: 700,
    color: '#94a3b8',
    textTransform: 'uppercase',
    letterSpacing: '0.06em',
  }
  const inputStyle: React.CSSProperties = {
    border: '1px solid #e2e8f0',
    borderRadius: 8,
    padding: '9px 12px',
    fontSize: 14,
    outline: 'none',
    color: '#1e293b',
    width: '100%',
  }

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title} size="md" footer={footer}>
      <style>{`@keyframes ecm-spin { to { transform: rotate(360deg); } }`}</style>
      {/* Paso 1: elegir cómo redactar */}
      {step === 'choose' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <p style={{ margin: 0, fontSize: 13, color: '#64748b' }}>
            Para <strong style={{ color: '#1e293b' }}>{recipient.email}</strong>. ¿Cómo quieres redactar
            el correo?
          </p>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <button
              onClick={loadTemplates}
              style={{
                display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: 8,
                padding: 18, borderRadius: 12, border: '1px solid #e2e8f0', background: '#fff',
                cursor: 'pointer', textAlign: 'left',
              }}
            >
              <FileText size={22} color="#7c3aed" />
              <span style={{ fontWeight: 700, fontSize: 14, color: '#1e293b' }}>Cargar plantilla</span>
              <span style={{ fontSize: 12, color: '#64748b' }}>
                Usa una plantilla guardada en Tools.
              </span>
            </button>
            <button
              onClick={chooseNew}
              style={{
                display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: 8,
                padding: 18, borderRadius: 12, border: '1px solid #e2e8f0', background: '#fff',
                cursor: 'pointer', textAlign: 'left',
              }}
            >
              <PenLine size={22} color="#7c3aed" />
              <span style={{ fontWeight: 700, fontSize: 14, color: '#1e293b' }}>Redactar nueva</span>
              <span style={{ fontSize: 12, color: '#64748b' }}>
                Empieza desde un correo en blanco.
              </span>
            </button>
          </div>
        </div>
      )}

      {/* Paso 2a: seleccionar plantilla */}
      {step === 'pick-template' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, minHeight: 120 }}>
          {loadingTemplates ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8, padding: 32, color: '#64748b' }}>
              <Loader2 size={18} style={{ animation: 'ecm-spin 1s linear infinite' }} /> Cargando plantillas…
            </div>
          ) : templates.length === 0 ? (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, padding: 32, color: '#94a3b8', textAlign: 'center' }}>
              <Inbox size={28} />
              <span style={{ fontSize: 13 }}>No hay plantillas guardadas todavía.</span>
              <Button variant="secondary" leftIcon={<PenLine size={15} />} onClick={chooseNew}>
                Redactar nueva
              </Button>
            </div>
          ) : (
            templates.map(t => (
              <button
                key={t.id}
                onClick={() => pickTemplate(t)}
                style={{
                  display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px',
                  borderRadius: 10, border: '1px solid #e2e8f0', background: '#fff',
                  cursor: 'pointer', textAlign: 'left', width: '100%',
                }}
              >
                <Mail size={18} color="#7c3aed" style={{ flexShrink: 0 }} />
                <span style={{ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0 }}>
                  <span style={{ fontWeight: 700, fontSize: 14, color: '#1e293b', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {t.title}
                  </span>
                  <span style={{ fontSize: 12, color: '#64748b', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {t.subject || 'Sin asunto'}
                  </span>
                </span>
              </button>
            ))
          )}
        </div>
      )}

      {/* Paso 2b/3: redactar */}
      {step === 'compose' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={labelStyle}>Para</label>
            <input style={{ ...inputStyle, background: '#f8fafc', color: '#64748b' }} value={`${recipient.name} · ${recipient.email}`} readOnly />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={labelStyle}>Asunto</label>
            <input style={inputStyle} value={subject} onChange={e => setSubject(e.target.value)} placeholder="Asunto del correo" />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={labelStyle}>Cuerpo</label>
            <textarea
              style={{ ...inputStyle, minHeight: 180, resize: 'vertical', lineHeight: 1.5, fontFamily: 'inherit' }}
              value={body}
              onChange={e => setBody(e.target.value)}
              placeholder="Escribe el mensaje…"
            />
          </div>
        </div>
      )}
    </Modal>
  )
}
