import { useState, useEffect, useCallback } from 'react'
import { RefreshCw } from 'lucide-react'
import { ticketService, WahaStatus as WahaStatusType } from '../../services/ticket.service'

type Kind = 'connected' | 'scan' | 'disconnected' | 'unknown'

function classify(status?: string): Kind {
  const s = (status || '').toUpperCase()
  if (s === 'WORKING' || s === 'CONNECTED') return 'connected'
  if (s.startsWith('SCAN')) return 'scan'
  if (s === 'STOPPED' || s === 'FAILED' || s === 'STARTING') return 'disconnected'
  return 'unknown'
}

const META: Record<Kind, { label: string; color: string; bg: string }> = {
  connected:    { label: 'Conectado',     color: '#128C7E', bg: 'rgba(37,211,102,0.12)' },
  scan:         { label: 'Escanear QR',   color: '#B45309', bg: 'rgba(245,158,11,0.15)' },
  disconnected: { label: 'Desconectado',  color: '#B91C1C', bg: 'rgba(239,68,68,0.12)' },
  unknown:      { label: 'Sin estado',    color: '#667781', bg: 'rgba(102,119,129,0.12)' },
}

function qrSrc(image?: string): string | null {
  if (!image) return null
  return image.startsWith('data:') ? image : `data:image/png;base64,${image}`
}

export default function WahaStatus() {
  const [status, setStatus] = useState<WahaStatusType | null>(null)
  const [loading, setLoading] = useState(true)
  const [forcing, setForcing] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const data = await ticketService.getWahaStatus()
      setStatus(data)
      setError('')
    } catch {
      setError('No se pudo consultar el estado de WhatsApp')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    const t = setInterval(load, 20000) // refresco cada 20s
    return () => clearInterval(t)
  }, [load])

  const handleForce = async () => {
    if (forcing) return
    setForcing(true)
    setError('')
    try {
      const data = await ticketService.forceWahaConnection()
      setStatus(data)
    } catch {
      setError('No se pudo forzar la conexión')
    } finally {
      setForcing(false)
    }
  }

  const kind = classify(status?.status)
  const meta = META[kind]
  const qr = kind === 'scan' ? qrSrc(status?.qr?.image) : null

  return (
    <div style={{
      padding: '8px 16px',
      borderBottom: '1px solid #e9edef',
      background: '#fafbfc',
      flexShrink: 0,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <span style={{
          width: 9, height: 9, borderRadius: '50%',
          background: kind === 'connected' ? '#25D366' : kind === 'scan' ? '#F59E0B' : '#EF4444',
          flexShrink: 0,
        }} />
        <span style={{
          fontSize: '12px', fontWeight: 600, color: meta.color,
          background: meta.bg, padding: '2px 8px', borderRadius: '10px',
        }}>
          {loading ? 'Cargando…' : meta.label}
        </span>

        <button
          onClick={handleForce}
          disabled={forcing}
          title="Reconectar con WhatsApp"
          style={{
            marginLeft: 'auto',
            display: 'flex', alignItems: 'center', gap: '6px',
            fontSize: '11px', fontWeight: 600,
            padding: '5px 10px', borderRadius: '14px',
            border: '1px solid #d1d7db', background: 'white',
            color: '#128C7E', cursor: forcing ? 'default' : 'pointer',
            opacity: forcing ? 0.6 : 1, fontFamily: 'inherit',
          }}
        >
          <RefreshCw size={13} className={forcing ? 'spin' : ''} style={forcing ? { animation: 'spin 1s linear infinite' } : {}} />
          {forcing ? 'Conectando…' : 'Reconectar'}
        </button>
      </div>

      {error && (
        <div style={{ marginTop: 6, fontSize: '11px', color: '#B91C1C' }}>{error}</div>
      )}

      {qr && (
        <div style={{ marginTop: 10, textAlign: 'center' }}>
          <p style={{ fontSize: '11px', color: '#667781', margin: '0 0 6px 0' }}>
            Escaneá este código desde WhatsApp → Dispositivos vinculados
          </p>
          <img
            src={qr}
            alt="Código QR de WhatsApp"
            style={{ width: 180, height: 180, borderRadius: 8, border: '1px solid #e9edef', background: 'white' }}
          />
        </div>
      )}

      <style>{`@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }`}</style>
    </div>
  )
}
