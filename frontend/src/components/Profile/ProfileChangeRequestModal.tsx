import React, { useRef, useState } from 'react'
import { Upload, FileCheck2, Eye, Loader2 } from 'lucide-react'
import { Modal, Button } from '../ui'
import { Select } from '../ui/Select'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../Auth/countries'
import { uploadService, profileChangeService } from '../../services/api'
import type { User } from '../../types'
import styles from '../../pages/Profile.module.css'

interface Props {
  user: User
  isOpen: boolean
  onClose: () => void
  onSubmitted: () => void
}

export function ProfileChangeRequestModal({ user, isOpen, onClose, onSubmitted }: Props) {
  const [isSaving, setIsSaving] = useState(false)
  const [isUploadingDoc, setIsUploadingDoc] = useState(false)
  const [error, setError] = useState('')
  const docInputRef = useRef<HTMLInputElement>(null)
  const [form, setForm] = useState({
    name: user.name || '',
    phone_number: user.phone_number || '',
    country: user.country || '',
    state: user.state || '',
    city: user.city || '',
    location: user.location || '',
    job_title: user.job_title || '',
    identity_document: user.identity_document || '',
  })
  const [note, setNote] = useState('')

  const countryOptions = !form.country || COUNTRY_OPTIONS.some(o => o.value === form.country)
    ? COUNTRY_OPTIONS
    : [{ value: form.country, label: form.country }, ...COUNTRY_OPTIONS]
  const baseStateOptions = getStatesForCountry(form.country)
  const stateOptions = !form.state || baseStateOptions.some(o => o.value === form.state)
    ? baseStateOptions
    : [{ value: form.state, label: form.state }, ...baseStateOptions]

  const handleDocUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const allowed = ['application/pdf', 'image/jpeg', 'image/png', 'image/webp', 'image/gif']
    if (!allowed.includes(file.type)) { setError('El documento debe ser un PDF o una imagen'); if (docInputRef.current) docInputRef.current.value = ''; return }
    if (file.size > 10 * 1024 * 1024) { setError('El documento debe ser menor a 10MB'); if (docInputRef.current) docInputRef.current.value = ''; return }
    setIsUploadingDoc(true); setError('')
    try {
      const result = await uploadService.upload(file)
      setForm(prev => ({ ...prev, identity_document: result.url }))
    } catch {
      setError('Error al subir el documento')
    } finally {
      setIsUploadingDoc(false)
      if (docInputRef.current) docInputRef.current.value = ''
    }
  }

  const handleSubmit = async () => {
    setIsSaving(true); setError('')
    try {
      await profileChangeService.create(form, note)
      onSubmitted()
      onClose()
    } catch (err: any) {
      setError(err?.response?.data?.error || err?.message || 'No se pudo enviar la solicitud')
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Solicitar actualización de datos"
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSaving}>Cancelar</Button>
          <Button variant="primary" onClick={handleSubmit} loading={isSaving} disabled={isUploadingDoc}>Enviar solicitud</Button>
        </>
      }
    >
      <p style={{ fontSize: '0.85rem', color: '#64748b', marginTop: 0 }}>
        Indica los valores que deseas actualizar. Tu solicitud se enviará a Customer Success, que la revisará y aplicará los cambios.
      </p>

      {error && <div className={`${styles['alert']} ${styles['error']}`}>{error}</div>}

      <div className={styles['form-row']}>
        <div className={styles['form-group']}>
          <label>Nombre completo</label>
          <input type="text" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} />
        </div>
        <div className={styles['form-group']}>
          <label>Teléfono</label>
          <input type="tel" value={form.phone_number} onChange={e => setForm({ ...form, phone_number: e.target.value })} />
        </div>
      </div>

      <div className={styles['form-row']}>
        <div className={styles['form-group']}>
          <label>País</label>
          <Select fullWidth value={form.country} onChange={v => setForm({ ...form, country: String(v), state: '' })} placeholder="Selecciona un país..." options={countryOptions} />
        </div>
        <div className={styles['form-group']}>
          <label>Provincia / Estado</label>
          <Select fullWidth value={form.state} onChange={v => setForm({ ...form, state: String(v) })} placeholder="Selecciona una provincia..." options={stateOptions} disabled={!form.country || stateOptions.length === 0} />
        </div>
      </div>

      <div className={styles['form-row']}>
        <div className={styles['form-group']}>
          <label>Ciudad</label>
          <input type="text" value={form.city} onChange={e => setForm({ ...form, city: e.target.value })} />
        </div>
        <div className={styles['form-group']}>
          <label>Puesto / Cargo</label>
          <input type="text" value={form.job_title} onChange={e => setForm({ ...form, job_title: e.target.value })} placeholder="Ej: Desarrollador Frontend" />
        </div>
      </div>

      <div className={styles['form-group']}>
        <label>Dirección</label>
        <textarea value={form.location} onChange={e => setForm({ ...form, location: e.target.value })} rows={2} placeholder="Dirección completa" />
      </div>

      <div className={styles['form-group']}>
        <label>Documento de identidad</label>
        <input ref={docInputRef} type="file" accept="application/pdf,image/*" onChange={handleDocUpload} style={{ display: 'none' }} />
        {form.identity_document ? (
          <div className={styles['doc-loaded']}>
            <FileCheck2 size={22} color="#10b981" style={{ flexShrink: 0 }} />
            <div className={styles['doc-loaded-info']}>
              <span className={styles['doc-loaded-title']}>Documento cargado</span>
              <a href={form.identity_document} target="_blank" rel="noopener noreferrer" className={styles['doc-loaded-link']}>
                <Eye size={14} /> Ver documento
              </a>
            </div>
            <button type="button" className={styles['doc-replace-btn']} onClick={() => docInputRef.current?.click()} disabled={isUploadingDoc}>
              {isUploadingDoc ? 'Subiendo…' : 'Reemplazar'}
            </button>
          </div>
        ) : (
          <button type="button" className={styles['doc-dropzone']} onClick={() => docInputRef.current?.click()} disabled={isUploadingDoc}>
            {isUploadingDoc ? <Loader2 size={26} className={styles['doc-spin']} /> : <Upload size={26} />}
            <span className={styles['doc-dropzone-title']}>{isUploadingDoc ? 'Subiendo documento…' : 'Haz clic para subir tu documento'}</span>
            <span className={styles['doc-dropzone-hint']}>PDF o imagen, hasta 10MB</span>
          </button>
        )}
      </div>

      <div className={styles['form-group']}>
        <label>Motivo (opcional)</label>
        <textarea value={note} onChange={e => setNote(e.target.value)} rows={2} placeholder="Ej: corregí mi número de teléfono" />
      </div>
    </Modal>
  )
}
