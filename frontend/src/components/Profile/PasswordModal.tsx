import React, { useState } from 'react'
import { userService } from '../../services/api'
import styles from '../../pages/Profile.module.css'
import { Modal, Button } from '../ui'

interface PasswordModalProps {
  isOpen: boolean
  onClose: () => void
  userId: number
}

export function PasswordModal({ isOpen, onClose, userId }: PasswordModalProps) {
  const [passwordData, setPasswordData] = useState({ current: '', new: '', confirm: '' })
  const [isLoading, setIsLoading] = useState(false)
  const [message, setMessage] = useState({ type: '', text: '' })

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault()
    if (passwordData.new !== passwordData.confirm) {
      setMessage({ type: 'error', text: 'Las contraseñas no coinciden' })
      return
    }
    setIsLoading(true)
    setMessage({ type: '', text: '' })

    try {
      await userService.changePassword(userId, passwordData.current, passwordData.new)
      setMessage({ type: 'success', text: 'Contraseña actualizada correctamente' })
      setTimeout(() => {
        onClose()
        setPasswordData({ current: '', new: '', confirm: '' })
        setMessage({ type: '', text: '' })
      }, 1500)
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || 'Error al cambiar la contraseña'
      setMessage({ type: 'error', text: errorMsg })
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Cambiar Contraseña"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button type="submit" form="password-form" loading={isLoading}>Cambiar</Button>
        </>
      }
    >
      {message.text && (
        <div className={`${styles['alert']} ${styles[message.type] || message.type}`} style={{ marginBottom: '16px' }}>
          {message.text}
        </div>
      )}

      <form id="password-form" onSubmit={handlePasswordChange}>
        <div className={styles['form-group']}>
          <label>Contraseña actual</label>
          <input
            type="password"
            value={passwordData.current}
            onChange={(e) => setPasswordData({ ...passwordData, current: e.target.value })}
            required
            autoFocus
          />
        </div>
        <div className={styles['form-group']}>
          <label>Nueva contraseña</label>
          <input
            type="password"
            value={passwordData.new}
            onChange={(e) => setPasswordData({ ...passwordData, new: e.target.value })}
            required
            minLength={6}
          />
        </div>
        <div className={styles['form-group']}>
          <label>Confirmar contraseña</label>
          <input
            type="password"
            value={passwordData.confirm}
            onChange={(e) => setPasswordData({ ...passwordData, confirm: e.target.value })}
            required
          />
        </div>
      </form>
    </Modal>
  )
}
