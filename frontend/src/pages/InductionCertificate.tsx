import { useEffect, useState } from 'react'
import { ArrowLeft, ArrowRight, Download } from 'lucide-react'

import type { CertificateSummary } from '../services/certificate.service'
import styles from './Induction.module.css'

interface Props {
  certificate: CertificateSummary
  /** A dónde lleva el botón principal y cómo se llama. */
  destination: { href: string; label: string }
  onBack: () => void
}

/**
 * Vista del certificado de la inducción: el PDF a la vista y, debajo, descargar
 * e ir a Obertrack.
 *
 * El servidor entrega el PDF como adjunto (Content-Disposition: attachment) y
 * nginx prohíbe enmarcar páginas propias, así que no se puede incrustar la URL
 * tal cual: se descarga como blob y se muestra desde ahí. En producción eso pide
 * `blob:` en `frame-src` de la CSP (nginx.conf.template).
 *
 * Si no se puede mostrar, quedan igualmente los botones de descargar e ir a la
 * plataforma: la vista previa es un extra, no un requisito.
 */
export default function InductionCertificate({ certificate, destination, onBack }: Props) {
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let objectUrl = ''
    let cancelled = false
    setPreviewUrl(null)
    setFailed(false)

    fetch(certificate.download_url)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.blob()
      })
      .then((blob) => {
        // El tipo debe ser PDF para que el navegador lo pinte en el marco.
        objectUrl = URL.createObjectURL(blob.type === 'application/pdf' ? blob : new Blob([blob], { type: 'application/pdf' }))
        if (cancelled) URL.revokeObjectURL(objectUrl)
        else setPreviewUrl(objectUrl)
      })
      .catch(() => {
        if (!cancelled) setFailed(true)
      })

    return () => {
      cancelled = true
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [certificate.download_url])

  return (
    <section className={styles.certView}>
      <h2 className={styles.stateTitle}>Tu certificado</h2>
      <p className={styles.stateText}>{certificate.program_name}</p>

      <div className={styles.certFrame}>
        {previewUrl ? (
          // #toolbar=0&navpanes=0&view=FitH: ajusta el ancho al marco sin márgenes laterales oscuros.
          <iframe src={`${previewUrl}#toolbar=0&navpanes=0&view=FitH`} title={`Certificado de ${certificate.program_name}`} />
        ) : failed ? (
          <p className={styles.certFrameMsg}>
            No pudimos mostrar la vista previa. Puedes descargar tu certificado con el botón de abajo.
          </p>
        ) : (
          <div className={styles.certFrameMsg}>
            <div className={styles.spinner} aria-hidden />
            <p>Cargando tu certificado...</p>
          </div>
        )}
      </div>

      <div className={styles.certActions}>
        <a className={styles.downloadBtn} href={certificate.download_url}>
          <Download size={18} /> Descargar certificado
        </a>
        <a className={styles.primaryBtn} href={destination.href}>
          {destination.label} <ArrowRight size={18} />
        </a>
      </div>

      <button type="button" className={styles.certBack} onClick={onBack}>
        <ArrowLeft size={15} /> Volver
      </button>
    </section>
  )
}
