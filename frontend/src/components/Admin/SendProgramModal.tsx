import { useEffect, useMemo, useState } from 'react'
import { Bell, Check, CheckCircle2, Info, Mail, Search, Send } from 'lucide-react'

import { Button, Modal } from '../ui'
import { useNotification } from '../../context/NotificationContext'
import {
  inductionService,
  type InductionProgram,
  type ProgramRecipient,
  type SendProgramResult,
} from '../../services/induction.service'
import styles from './InductionSettings.module.css'

interface Props {
  program: InductionProgram
  onClose: () => void
}

const SOURCE_LABEL: Record<ProgramRecipient['source'], string> = {
  persona: 'Asignado en persona',
  empresa: 'Por su empresa',
  por_defecto: 'Programa por defecto',
}

/**
 * Quien tiene otra CAPACITACIÓN en curso no puede recibir esta todavía. Un
 * ingreso en curso sí se puede reemplazar (sigue sin acceso hasta aprobar).
 */
const busy = (r: ProgramRecipient) => r.status === 'pending' && !r.pending_ingreso
/** Ingreso en curso: se puede, pero no se marca solo porque le cambia el programa. */
const replacesIngreso = (r: ProgramRecipient) => r.status === 'pending' && r.pending_ingreso

/**
 * Enviar un programa ahora a sus destinatarios. Llega como capacitación: quien
 * ya trabaja sigue trabajando; quien aún no tiene acceso lo tendrá al aprobar.
 * Por defecto no se marca ninguno para que el usuario elija a quién enviar.
 */
export default function SendProgramModal({ program, onClose }: Props) {
  const { success, error: showError } = useNotification()
  const [recipients, setRecipients] = useState<ProgramRecipient[] | null>(null)
  const [selected, setSelected] = useState<number[]>([])
  const [query, setQuery] = useState('')
  const [sending, setSending] = useState(false)
  const [result, setResult] = useState<SendProgramResult | null>(null)
  // Si el correo de invitación está encendido en Configuración → Correos.
  const [emailEnabled, setEmailEnabled] = useState(true)
  const [sendEmail, setSendEmail] = useState(true)

  useEffect(() => {
    let cancelled = false
    inductionService
      .programRecipients(program.id)
      .then(({ recipients: list, emailEnabled: enabled }) => {
        if (cancelled) return
        setRecipients(list)
        setEmailEnabled(enabled)
        setSelected([])
      })
      .catch(() => {
        if (!cancelled) setRecipients([])
      })
    return () => {
      cancelled = true
    }
  }, [program.id])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    const list = recipients ?? []
    return q ? list.filter((r) => `${r.name} ${r.email} ${r.company}`.toLowerCase().includes(q)) : list
  }, [recipients, query])

  const selectable = filtered.filter((r) => !busy(r))
  const allSelected = selectable.length > 0 && selectable.every((r) => selected.includes(r.user_id))

  const toggle = (id: number) =>
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))

  const handleSend = async () => {
    setSending(true)
    try {
      const res = await inductionService.sendProgram(program.id, selected, sendEmail && emailEnabled)
      setResult(res)
      if (res.sent > 0) success(`Inducción enviada a ${res.sent} ${res.sent === 1 ? 'profesional' : 'profesionales'}.`)
    } catch (err: any) {
      showError(err?.response?.data?.error ?? err?.message ?? 'No se pudo enviar.')
    } finally {
      setSending(false)
    }
  }

  const statusTag = (r: ProgramRecipient) => {
    if (busy(r)) return <span className={styles.tagWarn}>Capacitación en curso: {r.program_name || 'otra'}</span>
    if (replacesIngreso(r))
      return (
        <span className={styles.tagWarn} title="Su inducción de ingreso pasa a ser este programa. Sigue sin acceso hasta aprobarla.">
          Ingreso en curso ({r.program_name}): se reemplaza
        </span>
      )
    if (r.passed_this) return <span className={styles.tagOk}>Ya la completó</span>
    if (r.status === 'blocked') return <span className={styles.tagDanger}>Bloqueado en {r.program_name}</span>
    return null
  }

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={`Enviar «${program.name}»`}
      size="lg"
      footer={
        result ? (
          <Button onClick={onClose}>Listo</Button>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose} disabled={sending}>
              Cancelar
            </Button>
            <Button onClick={handleSend} loading={sending} disabled={selected.length === 0}>
              <Send size={15} /> Enviar a {selected.length}
            </Button>
          </>
        )
      }
    >
      {result ? (
        <div className={styles.sendResult}>
          <CheckCircle2 size={36} />
          <strong>
            {result.sent > 0
              ? `Enviada a ${result.sent} ${result.sent === 1 ? 'profesional' : 'profesionales'}`
              : 'No se envió a nadie'}
          </strong>
          <span>
            {result.emailed
              ? 'Les llegó por correo y en la campanita de la app.'
              : 'Les llegó en la campanita de la app (sin correo).'}
          </span>
          {result.failed.length > 0 && (
            <div className={styles.readinessBox} style={{ alignSelf: 'stretch', textAlign: 'left' }}>
              <strong style={{ fontSize: 13.5 }}>No se pudo enviar a {result.failed.length}:</strong>
              {result.failed.map((f) => (
                <div key={f.user_id} className={styles.readinessItem} data-level="warning">
                  <span>
                    {f.name || `Usuario ${f.user_id}`} <span className={styles.readinessFix}>— {f.reason}</span>
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : (
        <>
          <div className={styles.sendNote}>
            <Info size={16} />
            <span>
              Llega como <strong>capacitación</strong>: quien ya trabaja sigue trabajando normal mientras la
              hace. Quien todavía no tiene acceso a la plataforma lo tendrá al aprobarla.
            </span>
          </div>

          <div className={styles.sendChannels}>
            <span className={styles.sendChannel}>
              <Bell size={15} /> Campanita en la app
              <small>siempre</small>
            </span>
            <label className={emailEnabled ? styles.sendChannel : styles.sendChannelOff}>
              <input
                type="checkbox"
                checked={sendEmail && emailEnabled}
                disabled={!emailEnabled}
                onChange={(e) => setSendEmail(e.target.checked)}
              />
              <Mail size={15} /> También por correo
              <small>{emailEnabled ? 'con el enlace a la capacitación' : 'apagado en Configuración → Correos'}</small>
            </label>
          </div>

          {recipients === null ? (
            <p className={styles.muted}>Cargando destinatarios...</p>
          ) : recipients.length === 0 ? (
            <div className={styles.empty}>
              Este programa no tiene destinatarios. Asígnale empresas o profesionales en el paso
              «Destinatarios».
            </div>
          ) : (
            <>
              <div className={styles.pickToolbar}>
                <div className={styles.videoSearch} style={{ margin: 0, flex: 1, maxWidth: 360 }}>
                  <Search size={15} />
                  <input
                    type="text"
                    placeholder="Buscar por nombre, correo o empresa..."
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    aria-label="Buscar profesional"
                  />
                </div>
                <span className={styles.muted} style={{ fontSize: 13 }}>
                  {selected.length} de {recipients.length} elegidos
                </span>
                {selectable.length > 0 && (
                  <button
                    type="button"
                    className={styles.linkBtn}
                    onClick={() => {
                      const ids = selectable.map((r) => r.user_id)
                      setSelected((prev) =>
                        allSelected ? prev.filter((id) => !ids.includes(id)) : [...prev, ...ids.filter((id) => !prev.includes(id))]
                      )
                    }}
                  >
                    {allSelected ? 'Quitar los visibles' : 'Elegir los visibles'}
                  </button>
                )}
              </div>
              <div className={styles.sendList}>
                {filtered.map((r) => {
                  const on = selected.includes(r.user_id)
                  const disabled = busy(r)
                  return (
                    <button
                      key={r.user_id}
                      type="button"
                      role="checkbox"
                      aria-checked={on}
                      disabled={disabled}
                      title={
                        disabled
                          ? 'Tiene otra capacitación en curso: podrá recibir esta cuando la termine, o reiníciala desde Soporte.'
                          : undefined
                      }
                      className={on ? styles.sendRowOn : styles.sendRow}
                      onClick={() => toggle(r.user_id)}
                    >
                      <span className={styles.sendCheck}>{on && <Check size={13} strokeWidth={3} />}</span>
                      <span className={styles.pickText} style={{ flex: 1 }}>
                        <span className={styles.pickName}>{r.name}</span>
                        <span className={styles.pickSub}>
                          {r.company || 'Sin empresa'} · {SOURCE_LABEL[r.source]}
                        </span>
                      </span>
                      {statusTag(r)}
                    </button>
                  )
                })}
                {filtered.length === 0 && <p className={styles.hint}>Nadie coincide con «{query}».</p>}
              </div>
            </>
          )}
        </>
      )}
    </Modal>
  )
}
