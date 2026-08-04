import { useState, useEffect, useCallback } from 'react'
import { RefreshCw, DownloadCloud } from 'lucide-react'
import { ticketService, WahaStatus as WahaStatusType } from '../../services/ticket.service'

interface WahaStatusProps {
  // Se llama tras una traída manual que sí importó algo, para que la bandeja
  // muestre lo recién bajado sin esperar al sondeo de 30s.
  onSynced?: () => void
}

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

// Estilo común de los dos botones de la cabecera, para que no se separen al
// tocar uno solo.
const actionBtn: React.CSSProperties = {
  display: 'flex', alignItems: 'center', gap: '6px',
  fontSize: '11px', fontWeight: 600,
  padding: '5px 10px', borderRadius: '14px',
  border: '1px solid #d1d7db', background: 'white',
  color: '#128C7E', fontFamily: 'inherit',
}

function qrSrc(image?: string): string | null {
  if (!image) return null
  return image.startsWith('data:') ? image : `data:image/png;base64,${image}`
}

export default function WahaStatus({ onSynced }: WahaStatusProps) {
  const [status, setStatus] = useState<WahaStatusType | null>(null)
  const [loading, setLoading] = useState(true)
  const [forcing, setForcing] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [syncMsg, setSyncMsg] = useState('')
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

  // El import puede tardar bastante (WAHA remoto + varios chats), así que el
  // botón se bloquea mientras corre en vez de dejar disparar pasadas en paralelo.
  const handleSync = async () => {
    if (syncing) return
    setSyncing(true)
    setError('')
    setSyncMsg('')
    try {
      const { imported } = await ticketService.syncWahaHistory()
      setSyncMsg(imported > 0 ? `${imported} mensaje(s) nuevo(s)` : 'Todo al día')
      if (imported > 0) onSynced?.()
    } catch (err: any) {
      setSyncMsg(err?.response?.status === 409
        ? 'Ya hay una sincronización en curso'
        : 'No se pudieron traer los mensajes')
    } finally {
      setSyncing(false)
    }
  }

  // El aviso del resultado se borra solo: es información del momento, no un
  // estado que deba quedarse fijo en la cabecera.
  useEffect(() => {
    if (!syncMsg) return
    const t = setTimeout(() => setSyncMsg(''), 6000)
    return () => clearTimeout(t)
  }, [syncMsg])

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
  // El backend adjunta el QR en cualquier estado no conectado (SCAN_QR, FAILED,
  // STOPPED…). Mostrarlo solo en SCAN dejaba al operador sin forma de re-vincular
  // tras una caída, aun teniendo el código disponible.
  const qr = kind === 'connected' ? null : qrSrc(status?.qr?.image)

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
          onClick={handleSync}
          disabled={syncing}
          title="Traer los mensajes más recientes desde WhatsApp"
          style={{
            ...actionBtn,
            marginLeft: 'auto',
            cursor: syncing ? 'default' : 'pointer',
            opacity: syncing ? 0.6 : 1,
          }}
        >
          <DownloadCloud size={13} style={syncing ? { animation: 'spin 1s linear infinite' } : {}} />
          {syncing ? 'Trayendo…' : 'Traer mensajes'}
        </button>

        <button
          onClick={handleForce}
          disabled={forcing}
          title="Reconectar con WhatsApp"
          style={{
            ...actionBtn,
            cursor: forcing ? 'default' : 'pointer',
            opacity: forcing ? 0.6 : 1,
          }}
        >
          <RefreshCw size={13} className={forcing ? 'spin' : ''} style={forcing ? { animation: 'spin 1s linear infinite' } : {}} />
          {forcing ? 'Conectando…' : 'Reconectar'}
        </button>
      </div>

      {syncMsg && (
        <div style={{ marginTop: 6, fontSize: '11px', color: '#667781' }}>{syncMsg}</div>
      )}

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
