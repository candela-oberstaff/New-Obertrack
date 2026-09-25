import { useEffect, useState } from 'react'
import { FileCheck, Download, ShieldCheck } from 'lucide-react'

import { certificateService, certificateDownloadUrl, type Certificate } from '../../services/certificate.service'

interface Props {
  /** Sin userId = los del usuario de la sesión. */
  userId?: number
  className?: string
  style?: React.CSSProperties
  titleStyle?: React.CSSProperties
  /** Muestra la tarjeta aunque no haya certificados (perfil propio). */
  alwaysShow?: boolean
  emptyText?: string
  refreshKey?: unknown
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('es-ES', { day: '2-digit', month: 'long', year: 'numeric' })
}

/**
 * Certificados de una persona con descarga y código de verificación. Se
 * oculta sola si no hay ninguno, salvo que se pida lo contrario.
 */
export function UserCertificates({
  userId,
  className,
  style,
  titleStyle,
  alwaysShow = false,
  emptyText = 'Todavía no hay certificados. Se emiten al completar un programa que los tenga configurados.',
  refreshKey,
}: Props) {
  const [certs, setCerts] = useState<Certificate[] | null>(null)

  useEffect(() => {
    let cancelled = false
    const req = userId ? certificateService.forUser(userId) : certificateService.mine()
    req
      .then((list) => {
        if (!cancelled) setCerts(list ?? [])
      })
      .catch(() => {
        if (!cancelled) setCerts([])
      })
    return () => {
      cancelled = true
    }
  }, [userId, refreshKey])

  if (certs === null) return null
  if (certs.length === 0 && !alwaysShow) return null

  return (
    <div className={className} style={style}>
      <h3 style={{ display: 'flex', alignItems: 'center', gap: 8, ...titleStyle }}>
        <FileCheck size={18} /> Certificados
        <span
          style={{
            fontSize: 12,
            fontWeight: 700,
            padding: '2px 9px',
            borderRadius: 999,
            background: '#f1f5f9',
            color: '#64748b',
            marginLeft: 4,
          }}
        >
          {certs.length}
        </span>
      </h3>

      {certs.length === 0 ? (
        <p style={{ fontSize: 13.5, color: '#94a3b8', margin: 0, lineHeight: 1.55 }}>{emptyText}</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {certs.map((c) => (
            <div
              key={c.id}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 12,
                padding: '10px 12px',
                background: '#f8fafc',
                border: '1px solid #e2e8f0',
                borderRadius: 10,
                flexWrap: 'wrap',
              }}
            >
              <div style={{ flex: 1, minWidth: 160 }}>
                <div style={{ fontSize: 14, fontWeight: 700, color: '#0f172a' }}>{c.program_name}</div>
                <div style={{ fontSize: 12, color: '#64748b', display: 'flex', alignItems: 'center', gap: 6, marginTop: 2 }}>
                  <ShieldCheck size={12} /> {c.code} · {formatDate(c.issued_at)}
                </div>
              </div>
              <a
                href={certificateDownloadUrl(c.code)}
                download
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 6,
                  padding: '8px 12px',
                  borderRadius: 10,
                  border: '1px solid #e2e8f0',
                  background: 'white',
                  color: '#0f172a',
                  fontSize: 13,
                  fontWeight: 700,
                  textDecoration: 'none',
                }}
              >
                <Download size={14} /> Descargar
              </a>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
