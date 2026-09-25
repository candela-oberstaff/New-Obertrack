import { useCallback, useEffect, useState } from 'react'
import { Plus, Pencil, Trash2, FileCheck } from 'lucide-react'

import { useConfirm } from '../ui/ConfirmProvider'
import { useNotification } from '../../context/NotificationContext'
import { certificateService, templateImageUrl, type CertificateTemplate } from '../../services/certificate.service'
import CertificateTemplateEditor from './CertificateTemplateEditor'
import styles from '../Admin/InductionSettings.module.css'

interface Props {
  /** Avisa cuando cambia la biblioteca, para refrescar los selectores. */
  onChanged?: () => void
}

/**
 * Biblioteca de plantillas de certificado. Cada programa elige una; sin
 * plantilla, el programa no certifica.
 */
export default function CertificateTemplateList({ onChanged }: Props) {
  const { success, error: showError } = useNotification()
  const confirm = useConfirm()

  const [templates, setTemplates] = useState<CertificateTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<CertificateTemplate | 'new' | null>(null)

  const load = useCallback(async () => {
    try {
      setTemplates(await certificateService.listTemplates())
    } catch {
      setTemplates([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const handleDelete = async (t: CertificateTemplate) => {
    if (t.program_names.length > 0) {
      showError(`Esta plantilla está en uso en: ${t.program_names.join(', ')}. Quítala de esos programas antes de borrarla.`)
      return
    }
    const ok = await confirm({
      title: 'Borrar plantilla',
      message: `Se borrará la plantilla "${t.name}". Los certificados ya emitidos con ella no cambian.`,
      confirmLabel: 'Borrar',
    })
    if (!ok) return
    try {
      await certificateService.deleteTemplate(t.id)
      success('Plantilla borrada.')
      await load()
      onChanged?.()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo borrar la plantilla.')
    }
  }

  if (editing) {
    return (
      <CertificateTemplateEditor
        template={editing === 'new' ? null : editing}
        onSaved={async (saved) => {
          setEditing(saved)
          await load()
          onChanged?.()
        }}
        onBack={() => setEditing(null)}
      />
    )
  }

  return (
    <div>
      <div className={styles.toolbar}>
        <p className={styles.intro}>
          Una plantilla es un diseño del equipo (imagen A4) con los campos que se imprimen encima. Cada programa
          elige la suya; al completarlo se emite el certificado con código de verificación.
        </p>
        <button type="button" className={styles.saveBtn} onClick={() => setEditing('new')}>
          <Plus size={16} /> Nueva plantilla
        </button>
      </div>

      {loading ? (
        <p className={styles.muted}>Cargando plantillas...</p>
      ) : templates.length === 0 ? (
        <div className={styles.empty}>
          Todavía no hay plantillas. Sube el primer diseño para que los programas puedan certificar.
          <div style={{ marginTop: 12 }}>
            <button type="button" className={styles.ghostBtn} onClick={() => setEditing('new')}>
              <Plus size={16} /> Crear la primera plantilla
            </button>
          </div>
        </div>
      ) : (
        <div className={styles.list}>
          {templates.map((t) => (
            <div key={t.id} className={styles.row}>
              <div
                style={{
                  width: t.orientation === 'L' ? 64 : 46,
                  height: t.orientation === 'L' ? 46 : 64,
                  borderRadius: 8,
                  overflow: 'hidden',
                  border: '1px solid #e2e8f0',
                  background: '#f8fafc',
                  flexShrink: 0,
                }}
              >
                <img
                  src={templateImageUrl(t.image_filename)}
                  alt=""
                  style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                />
              </div>
              <div className={styles.rowMain}>
                <div className={styles.rowTitle}>
                  {t.name}
                  <span className={styles.tag}>{t.orientation === 'L' ? 'Horizontal' : 'Vertical'}</span>
                </div>
                <div className={styles.rowMeta}>
                  <span className={styles.tag}>
                    <FileCheck size={12} style={{ verticalAlign: -2, marginRight: 4 }} />
                    {t.fields.length} {t.fields.length === 1 ? 'campo' : 'campos'}
                  </span>
                  {t.program_names.length > 0 ? (
                    <span>En: {t.program_names.join(', ')}</span>
                  ) : (
                    <span>Sin usar en ningún programa</span>
                  )}
                </div>
              </div>
              <div className={styles.rowActions}>
                <button
                  type="button"
                  className={`${styles.iconBtn} ${styles.iconBtnNeutral}`}
                  title="Editar"
                  aria-label={`Editar ${t.name}`}
                  onClick={() => setEditing(t)}
                >
                  <Pencil size={16} />
                </button>
                <button
                  type="button"
                  className={styles.iconBtn}
                  title={t.program_names.length > 0 ? 'En uso: quítala de sus programas para borrarla' : 'Borrar'}
                  aria-label={`Borrar ${t.name}`}
                  disabled={t.program_names.length > 0}
                  onClick={() => handleDelete(t)}
                >
                  <Trash2 size={16} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
