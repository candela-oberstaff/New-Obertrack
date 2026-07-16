import { useState } from 'react'
import { Check, X, Inbox } from 'lucide-react'
import type { BoardInvitation } from '../../../types'
import { Modal, Button } from '../../ui'

interface BoardInvitationsModalProps {
  isOpen: boolean
  onClose: () => void
  invitations: BoardInvitation[]
  onAccept: (invId: number) => Promise<void>
  onReject: (invId: number) => Promise<void>
}

export function BoardInvitationsModal({
  isOpen,
  onClose,
  invitations,
  onAccept,
  onReject,
}: BoardInvitationsModalProps) {
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
    <Modal isOpen={isOpen} onClose={onClose} title="Invitaciones a tableros" size="md">
      {invitations.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '40px 16px', color: '#94a3b8' }}>
          <Inbox size={40} style={{ marginBottom: 10 }} />
          <p style={{ margin: 0, fontSize: 14 }}>No tenés invitaciones pendientes.</p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {invitations.map((inv) => (
            <div
              key={inv.id}
              style={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12,
                padding: '12px 14px', border: '1px solid #e2e8f0', borderRadius: 12, flexWrap: 'wrap',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
                <span
                  style={{ width: 10, height: 10, borderRadius: 999, background: inv.board?.color || '#7c3aed', flexShrink: 0 }}
                />
                <div style={{ minWidth: 0 }}>
                  <div style={{ color: '#0f172a', fontSize: 14 }}>
                    <strong>{inv.inviter?.name || inv.board?.creator?.name || 'Un responsable'}</strong>
                    {' te invitó al tablero '}
                    <strong>{inv.board?.name || `#${inv.board_id}`}</strong>
                  </div>
                  <div style={{ fontSize: 12, color: '#94a3b8' }}>¿Querés unirte?</div>
                </div>
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Button
                  size="sm"
                  variant="secondary"
                  leftIcon={<X size={15} />}
                  loading={busyId === inv.id}
                  onClick={() => run(inv.id, onReject)}
                >
                  Rechazar
                </Button>
                <Button
                  size="sm"
                  leftIcon={<Check size={15} />}
                  loading={busyId === inv.id}
                  onClick={() => run(inv.id, onAccept)}
                >
                  Aceptar
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </Modal>
  )
}
