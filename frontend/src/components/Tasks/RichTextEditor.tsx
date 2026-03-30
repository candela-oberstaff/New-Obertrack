import { } from 'react'
import styles from './RichTextEditor.module.css'

interface RichTextEditorProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

export function RichTextEditor({ value, onChange, placeholder }: RichTextEditorProps) {
  return (
    <div className={styles.richTextEditor}>
      <div className={styles.richTextToolbar}>
        <button type="button" onClick={() => {
          const selection = window.getSelection()?.toString() || ''
          if (selection) onChange(value + `<strong>${selection}</strong>`)
        }} title="Negrita">
          <strong>B</strong>
        </button>
        <button type="button" onClick={() => {
          const selection = window.getSelection()?.toString() || ''
          if (selection) onChange(value + `<em>${selection}</em>`)
        }} title="Cursiva">
          <em>I</em>
        </button>
        <button type="button" onClick={() => {
          const selection = window.getSelection()?.toString() || ''
          if (selection) onChange(value + `<u>${selection}</u>`)
        }} title="Subrayado">
          <u>U</u>
        </button>
        <span className={styles.toolbarSeparator}>|</span>
        <button type="button" onClick={() => onChange(value + '<ul>\n  <li>Elemento 1</li>\n  <li>Elemento 2</li>\n</ul>')} title="Viñetas">
          •
        </button>
        <button type="button" onClick={() => onChange(value + '<ol>\n  <li>Elemento 1</li>\n  <li>Elemento 2</li>\n</ol>')} title="Numeración">
          1.
        </button>
        <span className={styles.toolbarSeparator}>|</span>
        <button type="button" onClick={() => {
          const url = prompt('Ingresa la URL:')
          if (url) onChange(value + `<a href="${url}">${url}</a>`)
        }} title="Enlace">
          Link
        </button>
        <button type="button" onClick={() => onChange(value + '<h2>Título</h2>')} title="Título">
          H2
        </button>
        <button type="button" onClick={() => onChange(value + '<p>Párrafo</p>')} title="Párrafo">
          P
        </button>
      </div>
      <textarea
        className={styles.richTextContent}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder || 'Escribe aquí...'}
        rows={8}
      />
    </div>
  )
}
