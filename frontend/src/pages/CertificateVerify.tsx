import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { ShieldCheck, ShieldX, Download } from 'lucide-react'

import { certificateService, type CertificateVerification } from '../services/certificate.service'
import styles from './Induction.module.css'

/**
 * Verificación pública de un certificado por su código. Es lo que le da
 * valor al documento fuera de Obertrack: cualquiera con el código (impreso en
 * el PDF) puede confirmar que se emitió, a quién y cuándo. No requiere sesión.
 */
export default function CertificateVerify() {
  const { code = '' } = useParams<{ code: string }>()
  const [result, setResult] = useState<CertificateVerification | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    certificateService
      .verify(code)
      .then((r) => {
        if (!cancelled) setResult(r)
      })
      .catch(() => {
        if (!cancelled) setResult({ valid: false, code, professional_name: '', program_name: '', issued_at: '', download_url: '' })
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [code])

  let content: React.ReactNode
  if (loading) {
    content = (
      <div className={styles.state}>
        <div className={styles.spinner} aria-hidden />
        <p className={styles.stateText}>Verificando el certificado...</p>
      </div>
    )
  } else if (!result || !result.valid) {
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconWarn}`}>
          <ShieldX size={30} />
        </div>
        <h2 className={styles.stateTitle}>Certificado no encontrado</h2>
        <p className={styles.stateText}>
          El código <strong>{code}</strong> no corresponde a ningún certificado emitido por Obertrack. Revisa que esté
          escrito tal como aparece en el documento.
        </p>
      </div>
    )
  } else {
    content = (
      <div className={styles.state}>
        <div className={`${styles.stateIcon} ${styles.iconOk}`}>
          <ShieldCheck size={30} />
        </div>
        <h2 className={styles.stateTitle}>Certificado válido</h2>
        <p className={styles.stateText}>
          Obertrack certifica que <strong>{result.professional_name}</strong> completó el programa{' '}
          <strong>{result.program_name}</strong> el{' '}
          {new Date(result.issued_at).toLocaleDateString('es-ES', { day: 'numeric', month: 'long', year: 'numeric' })}.
        </p>
        <p className={styles.stateText} style={{ fontFamily: 'monospace' }}>
          Código: {result.code}
        </p>
        <a className={styles.primaryBtn} href={result.download_url}>
          <Download size={18} /> Descargar PDF
        </a>
      </div>
    )
  }

  return (
    <div className={styles.page}>
      <div className={styles.shell}>
        <aside className={styles.brand}>
          <div className={styles.brandTop}>
            <img src="/logos/Horizontal_Blanco.png" alt="Obertrack" className={styles.logo} />
          </div>
          <div className={styles.brandBody}>
            <p className={styles.brandKicker}>Verificación de certificados</p>
            <h1 className={styles.brandName}>¿Es auténtico?</h1>
            <p className={styles.brandText}>
              Cada certificado emitido por Obertrack lleva un código único. Esta página confirma si existe y a quién
              se le otorgó.
            </p>
          </div>
          <p className={styles.brandFoot}>Obertrack · Remote Work Tracking</p>
        </aside>
        <main className={styles.content}>{content}</main>
      </div>
    </div>
  )
}
