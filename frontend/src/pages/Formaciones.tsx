import { useState } from 'react'
import { ArrowLeft, Clapperboard } from 'lucide-react'

import InductionSettings from '../components/Admin/InductionSettings'
import InductionVideoLibrary from '../components/Admin/InductionVideoLibrary'
import styles from './Tutoriales.module.css'

/**
 * Formaciones: la inducción de ingreso y las capacitaciones (bloques,
 * programas, certificados y su biblioteca de videos). Antes era una pestaña de
 * Novedades; ahora es su propia sección, solo para superadmin.
 */
export default function Formaciones() {
  // Los videos de las formaciones viven en su propia biblioteca.
  const [videoLibrary, setVideoLibrary] = useState(false)

  return (
    <div className={styles['tutorials-page']}>
      <header className={styles['tutorials-header']}>
        <div>
          <h1>Formaciones</h1>
          <p className={styles['tutorials-subtitle']}>
            Inducción de ingreso y capacitaciones: bloques, programas, certificados y videos.
          </p>
        </div>
        <div className={styles['tutorials-header-actions']}>
          <button type="button" className={styles['tutorials-create-btn']} onClick={() => setVideoLibrary((v) => !v)}>
            {videoLibrary ? <ArrowLeft size={18} /> : <Clapperboard size={18} />}
            {videoLibrary ? 'Volver a Formaciones' : 'Biblioteca de videos'}
          </button>
        </div>
      </header>

      {videoLibrary ? <InductionVideoLibrary onBack={() => setVideoLibrary(false)} /> : <InductionSettings />}
    </div>
  )
}
