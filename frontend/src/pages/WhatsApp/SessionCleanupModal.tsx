import { useCallback, useEffect, useState } from 'react'
import { AlertTriangle, Trash2, X } from 'lucide-react'
import { ticketService, WahaSessionInfo } from '../../services/ticket.service'

interface Props {
  onClose: () => void
  // Se llama tras un borrado, para que la bandeja deje de mostrar lo borrado.
  onPurged: () => void
}

const overlay: React.CSSProperties = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.45)',
  display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
}

const card: React.CSSProperties = {
  background: 'white', borderRadius: 12, padding: '20px 22px',
  width: 'min(560px, calc(100vw - 32px))', maxHeight: '80vh', overflowY: 'auto',
  boxShadow: '0 12px 40px rgba(0,0,0,0.2)',
}

/**
 * Borrado definitivo de las conversaciones de una sesión de WhatsApp.
 *
 * Existe por privacidad: al desvincular un número, sus conversaciones —que
 * pueden ser personales— se quedaban guardadas. Las de una sesión que ya no es
 * la activa ni siquiera se ven en la bandeja, pero siguen enteras en la base,
 * así que aquí se listan todas, incluidas esas.
 *
 * No hay deshacer, así que confirmar exige escribir el nombre de la sesión: es
 * la misma barrera que usa GitHub para borrar un repositorio, y evita el clic
 * accidental sobre la sesión equivocada.
 */
export default function SessionCleanupModal({ onClose, onPurged }: Props) {
  const [sessions, setSessions] = useState<WahaSessionInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [confirming, setConfirming] = useState<string | null>(null)
  const [typed, setTyped] = useState('')
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState('')

  const load = useCallback(async () => {
    try {
      setSessions(await ticketService.getWahaSessions())
      setError('')
    } catch {
      setError('No se pudieron cargar las sesiones guardadas')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const startConfirm = (session: string) => {
    setConfirming(session)
    setTyped('')
    setDone('')
    setError('')
  }

  const purge = async (session: string) => {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      const c = await ticketService.purgeWahaSession(session)
      setDone(`Se borraron ${c.tickets} conversación(es), ${c.messages} mensaje(s) y ${c.contacts} contacto(s).`)
      setConfirming(null)
      setTyped('')
      await load()
      onPurged()
    } catch (err: any) {
      setError(err?.response?.data?.error || 'No se pudieron borrar las conversaciones')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={overlay} onClick={onClose}>
      <div style={card} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
          <Trash2 size={17} color="#B91C1C" />
          <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700 }}>Conversaciones guardadas</h3>
          <button
            onClick={onClose}
            style={{ marginLeft: 'auto', border: 'none', background: 'none', cursor: 'pointer', color: '#667781' }}
            aria-label="Cerrar"
          >
            <X size={18} />
          </button>
        </div>

        <p style={{ fontSize: 12.5, color: '#667781', margin: '0 0 14px 0', lineHeight: 1.5 }}>
          Al desvincular un número, sus conversaciones siguen guardadas aunque dejen de verse en
          la bandeja. Borrarlas es <strong>definitivo</strong>: se eliminan de la base de datos y
          no hay forma de recuperarlas.
        </p>

        {loading && <p style={{ fontSize: 13, color: '#667781' }}>Cargando…</p>}

        {!loading && sessions.length === 0 && (
          <p style={{ fontSize: 13, color: '#667781' }}>No hay conversaciones de WhatsApp guardadas.</p>
        )}

        {sessions.map(s => (
          <div
            key={s.session}
            style={{
              border: '1px solid #e9edef', borderRadius: 8,
              padding: '10px 12px', marginBottom: 8,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              <strong style={{ fontSize: 13.5 }}>{s.session || '(sin nombre)'}</strong>
              {s.current ? (
                <span style={{
                  fontSize: 10.5, fontWeight: 700, padding: '2px 7px', borderRadius: 9,
                  color: '#128C7E', background: 'rgba(37,211,102,0.14)',
                }}>
                  Sesión actual
                </span>
              ) : (
                <span style={{
                  fontSize: 10.5, fontWeight: 700, padding: '2px 7px', borderRadius: 9,
                  color: '#B45309', background: 'rgba(245,158,11,0.16)',
                }}>
                  Número desvinculado
                </span>
              )}
              <span style={{ fontSize: 12, color: '#667781', marginLeft: 'auto' }}>
                {s.tickets} conversación(es) · {s.messages} mensaje(s)
              </span>
            </div>

            {confirming === s.session ? (
              <div style={{ marginTop: 10 }}>
                <div style={{
                  display: 'flex', alignItems: 'flex-start', gap: 6,
                  fontSize: 12, color: '#B91C1C', marginBottom: 8, lineHeight: 1.45,
                }}>
                  <AlertTriangle size={14} style={{ flexShrink: 0, marginTop: 1 }} />
                  <span>
                    Esto borra {s.messages} mensaje(s) para siempre.
                    Escribí <strong>{s.session}</strong> para confirmar.
                  </span>
                </div>
                <div style={{ display: 'flex', gap: 8 }}>
                  <input
                    autoFocus
                    value={typed}
                    onChange={e => setTyped(e.target.value)}
                    placeholder={s.session}
                    style={{
                      flex: 1, fontSize: 13, padding: '6px 9px',
                      border: '1px solid #d1d7db', borderRadius: 6, fontFamily: 'inherit',
                    }}
                  />
                  <button
                    onClick={() => purge(s.session)}
                    disabled={busy || typed !== s.session}
                    style={{
                      fontSize: 12, fontWeight: 700, padding: '6px 12px', borderRadius: 6,
                      border: 'none', color: 'white',
                      background: typed === s.session ? '#B91C1C' : '#e5b4b4',
                      cursor: busy || typed !== s.session ? 'default' : 'pointer',
                      fontFamily: 'inherit',
                    }}
                  >
                    {busy ? 'Borrando…' : 'Borrar'}
                  </button>
                  <button
                    onClick={() => setConfirming(null)}
                    disabled={busy}
                    style={{
                      fontSize: 12, fontWeight: 600, padding: '6px 12px', borderRadius: 6,
                      border: '1px solid #d1d7db', background: 'white', color: '#3b4a54',
                      cursor: 'pointer', fontFamily: 'inherit',
                    }}
                  >
                    Cancelar
                  </button>
                </div>
              </div>
            ) : (
              <button
                onClick={() => startConfirm(s.session)}
                style={{
                  marginTop: 8, fontSize: 12, fontWeight: 600,
                  padding: '5px 10px', borderRadius: 6,
                  border: '1px solid #f0c9c9', background: 'white', color: '#B91C1C',
                  cursor: 'pointer', fontFamily: 'inherit',
                  display: 'inline-flex', alignItems: 'center', gap: 6,
                }}
              >
                <Trash2 size={12} /> Borrar conversaciones
              </button>
            )}
          </div>
        ))}

        {done && <p style={{ fontSize: 12.5, color: '#128C7E', marginTop: 10 }}>{done}</p>}
        {error && <p style={{ fontSize: 12.5, color: '#B91C1C', marginTop: 10 }}>{error}</p>}
      </div>
    </div>
  )
}
