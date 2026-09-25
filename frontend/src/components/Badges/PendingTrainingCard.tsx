import { useEffect, useState } from 'react'
import { GraduationCap, ArrowRight } from 'lucide-react'

import { inductionService, type MyInduction } from '../../services/induction.service'

interface Props {
  className?: string
  style?: React.CSSProperties
}

/**
 * Aviso de capacitación pendiente en el perfil. Quien ya trabaja no recibe
 * el bloqueo de acceso, así que necesita ver desde dentro que tiene bloques
 * por completar y un botón para ir a hacerlos. Se oculta si no hay ninguna.
 */
export function PendingTrainingCard({ className, style }: Props) {
  const [data, setData] = useState<MyInduction | null>(null)

  useEffect(() => {
    let cancelled = false
    inductionService
      .myInduction()
      .then((d) => {
        if (!cancelled) setData(d && d.pending ? d : null)
      })
      .catch(() => {
        if (!cancelled) setData(null)
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (!data) return null

  const left = data.total_blocks - data.completed_blocks

  return (
    <div className={className} style={{ ...style, borderColor: 'rgba(204, 51, 204, 0.35)' }}>
      <h3 style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
        <GraduationCap size={18} /> Capacitación pendiente
      </h3>
      <p style={{ margin: '0 0 4px', fontSize: 14, fontWeight: 700, color: '#0f172a' }}>{data.program_name}</p>
      <p style={{ margin: '0 0 14px', fontSize: 13, color: '#64748b', lineHeight: 1.5 }}>
        {data.completed_blocks} de {data.total_blocks} bloques completados.{' '}
        {left === 1 ? 'Te falta 1 bloque' : `Te faltan ${left} bloques`} para ganar tus insignias.
      </p>
      <a
        href={`/induccion/${data.token}`}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 8,
          padding: '10px 16px',
          borderRadius: 12,
          background: 'linear-gradient(135deg, #cc33cc, #7a1f7a)',
          color: 'white',
          fontWeight: 700,
          fontSize: 14,
          textDecoration: 'none',
        }}
      >
        {data.completed_blocks > 0 ? 'Continuar' : 'Empezar'} <ArrowRight size={16} />
      </a>
    </div>
  )
}
