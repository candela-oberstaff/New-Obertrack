import { useState } from 'react'
import { Check, X, UserCheck } from 'lucide-react'
import type { BoardInvitation } from '../../../types'
import { Modal, Button } from '../../ui'

interface BoardRequestsModalProps {
  isOpen: boolean
  onClose: () => void
  boardName?: string
  requests: BoardInvitation[]
  onApprove: (invId: number) => Promise<void>
  onReject: (invId: number) => Promise<void>
}

const initials = (name: string) =>
  name.split(' ').map((n) => n[0]).join('').substring(0, 2).toUpperCase()

export function BoardRequestsModal({
  isOpen,
  onClose,
  boardName,
  requests,
  onApprove,
  onReject,
}: BoardRequestsModalProps) {
  const [busyId, setBusyId] = useState<number | null>(null)

  const run = async (invId: number, fn: (id: number) => Promise<void>) => {
    setBusyId(invId)
    try {
      await fn(invId)
    } finally {
      setBusyId(null)
    }
  }

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Solicitudes para unirse" size="md">
      {boardName && (
        <p style={{ color: '#64748b', marginTop: 0, marginBottom: 16 }}>{boardName}</p>
      )}
      {requests.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '40px 16px', color: '#94a3b8' }}>
          <UserCheck size={40} style={{ marginBottom: 10 }} />
          <p style={{ margin: 0, fontSize: 14 }}>No hay solicitudes pendientes.</p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {requests.map((req) => (
            <div
              key={req.id}
              style={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12,
                padding: '12px 14px', border: '1px solid #e2e8f0', borderRadius: 12, flexWrap: 'wrap',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
                <span
                  style={{
                    width: 36, height: 36, borderRadius: 999, background: '#ede9fe', color: '#6d28d9',
                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                    fontWeight: 800, fontSize: 12, flexShrink: 0,
                  }}
                >
                  {initials(req.user?.name || '?')}
                </span>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontWeight: 700, color: '#0f172a' }}>{req.user?.name || `Usuario #${req.user_id}`}</div>
                  <div style={{ fontSize: 12, color: '#94a3b8' }}>{req.user?.email}</div>
                </div>
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Button
                  size="sm"
                  variant="secondary"
                  leftIcon={<X size={15} />}
                  loading={busyId === req.id}
                  onClick={() => run(req.id, onReject)}
                >
                  Rechazar
                </Button>
                <Button
                  size="sm"
                  leftIcon={<Check size={15} />}
                  loading={busyId === req.id}
                  onClick={() => run(req.id, onApprove)}
                >
                  Aprobar
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </Modal>
  )
}
