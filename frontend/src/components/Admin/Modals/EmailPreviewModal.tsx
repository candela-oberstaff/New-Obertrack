import { useState, useEffect } from 'react'
import { Eye, X, Mail, Loader2 } from 'lucide-react'
import { settingsService, type EmailType } from '../../../services/settings.service'

interface EmailPreviewModalProps {
  emailType: EmailType | null
  onClose: () => void
}

export function EmailPreviewModal({ emailType, onClose }: EmailPreviewModalProps) {
  if (!emailType) return null

  const [loading, setLoading] = useState(true)
  const [previewData, setPreviewData] = useState<{ subject: string; body: string } | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setLoading(true)
    setError(null)
    settingsService
      .getEmailPreview(emailType.key)
      .then((data) => {
        setPreviewData({ subject: data.subject, body: data.body })
      })
      .catch((err) => {
        setError(err?.response?.data?.error || 'No se pudo cargar la vista previa del correo.')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [emailType.key])

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(15, 23, 42, 0.65)',
        backdropFilter: 'blur(4px)',
        zIndex: 9999,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 16,
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: '#ffffff',
          width: '100%',
          maxWidth: 700,
          borderRadius: 20,
          boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
          overflow: 'hidden',
          display: 'flex',
          flexDirection: 'column',
          maxHeight: '90vh',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Cabecera del modal */}
        <div
          style={{
            padding: '18px 24px',
            borderBottom: '1px solid #e2e8f0',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            background: 'linear-gradient(to right, #f8fafc, #ffffff)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 10,
                background: '#e0e7ff',
                color: '#4f46e5',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Eye size={18} />
            </div>
            <div>
              <h3 style={{ margin: 0, fontSize: 16, fontWeight: 800, color: '#0f172a' }}>
                Vista previa: {emailType.name}
              </h3>
              <span style={{ fontSize: 12, color: '#64748b' }}>
                Formato real de salida generado por Obertrack
              </span>
            </div>
          </div>

          <button
            onClick={onClose}
            style={{
              background: '#f1f5f9',
              border: 'none',
              borderRadius: 10,
              padding: 8,
              cursor: 'pointer',
              color: '#64748b',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <X size={18} />
          </button>
        </div>

        {/* Encabezado del mensaje simulación cliente de correo */}
        <div
          style={{
            background: '#f8fafc',
            borderBottom: '1px solid #e2e8f0',
            padding: '14px 24px',
            display: 'flex',
            flexDirection: 'column',
            gap: 6,
            fontSize: 13,
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ fontWeight: 700, color: '#64748b', width: 65 }}>De:</span>
            <span style={{ color: '#0f172a', fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
              <Mail size={14} style={{ color: '#6366f1' }} /> Obertrack &lt;notificaciones@obertrack.com&gt;
            </span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ fontWeight: 700, color: '#64748b', width: 65 }}>Asunto:</span>
            <span style={{ color: '#0f172a', fontWeight: 700 }}>
              {loading ? 'Cargando asunto…' : previewData?.subject || 'Sin asunto'}
            </span>
          </div>
        </div>

        {/* Cuerpo del correo */}
        <div
          style={{
            padding: 24,
            overflowY: 'auto',
            flex: 1,
            background: '#ffffff',
          }}
        >
          {loading ? (
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                padding: '48px 0',
                gap: 12,
                color: '#64748b',
              }}
            >
              <Loader2 size={24} style={{ animation: 'spin 1s linear infinite' }} />
              <span style={{ fontSize: 13, fontWeight: 600 }}>Cargando vista previa…</span>
            </div>
          ) : error ? (
            <div
              style={{
                padding: '16px 20px',
                background: '#fef2f2',
                border: '1px solid #fecaca',
                borderRadius: 12,
                color: '#b91c1c',
                fontSize: 13,
              }}
            >
              {error}
            </div>
          ) : (
            <div
              style={{
                border: '1px solid #e2e8f0',
                borderRadius: 14,
                padding: '24px',
                background: '#ffffff',
                boxShadow: '0 1px 3px rgba(0,0,0,0.05)',
                fontFamily: 'Inter, system-ui, -apple-system, sans-serif',
              }}
              dangerouslySetInnerHTML={{ __html: previewData?.body || '' }}
            />
          )}
        </div>

        {/* Pie de página */}
        <div
          style={{
            padding: '14px 24px',
            borderTop: '1px solid #e2e8f0',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-end',
            background: '#f8fafc',
          }}
        >
          <button
            onClick={onClose}
            style={{
              padding: '8px 18px',
              borderRadius: 10,
              border: '1px solid #cbd5e1',
              background: '#ffffff',
              color: '#334155',
              fontWeight: 700,
              fontSize: 13,
              cursor: 'pointer',
            }}
          >
            Cerrar
          </button>
        </div>
      </div>
    </div>
  )
}
