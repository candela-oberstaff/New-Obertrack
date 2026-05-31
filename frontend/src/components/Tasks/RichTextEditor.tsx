import { useRef, useEffect, useCallback } from 'react'
import { Bold, Italic, Underline, List, ListOrdered, Link as LinkIcon, Heading2, Type } from 'lucide-react'
import { sanitizeHtml } from '../../utils/sanitize'
import styles from './RichTextEditor.module.css'

interface RichTextEditorProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

export function RichTextEditor({ value, onChange, placeholder }: RichTextEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null)
  const isInternalChange = useRef(false)

  // Sync value from prop to innerHTML
  useEffect(() => {
    if (editorRef.current && !isInternalChange.current) {
      const safe = sanitizeHtml(value)
      if (editorRef.current.innerHTML !== safe) {
        editorRef.current.innerHTML = safe
      }
    }
    isInternalChange.current = false
  }, [value])

  const handleInput = useCallback(() => {
    if (editorRef.current) {
      const html = editorRef.current.innerHTML
      isInternalChange.current = true
      onChange(html === '<br>' ? '' : html)
    }
  }, [onChange])

  const execCommand = (command: string, arg?: string) => {
    document.execCommand(command, false, arg)
    handleInput()
    editorRef.current?.focus()
  }

  // Handle keyboard shortcuts
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.ctrlKey || e.metaKey) {
      if (e.key === 'b') { e.preventDefault(); execCommand('bold'); }
      if (e.key === 'i') { e.preventDefault(); execCommand('italic'); }
      if (e.key === 'u') { e.preventDefault(); execCommand('underline'); }
    }
  }

  return (
    <div className={styles.richTextEditor}>
      <div className={styles.richTextToolbar}>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('bold'); }} title="Negrita">
          <Bold size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('italic'); }} title="Cursiva">
          <Italic size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('underline'); }} title="Subrayado">
          <Underline size={16} />
        </button>
        <span className={styles.toolbarSeparator}>|</span>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('insertUnorderedList'); }} title="Viñetas">
          <List size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('insertOrderedList'); }} title="Numeración">
          <ListOrdered size={16} />
        </button>
        <span className={styles.toolbarSeparator}>|</span>
        <button type="button" onMouseDown={(e) => {
          e.preventDefault()
          const url = prompt('Ingresa la URL:')
          if (url) execCommand('createLink', url)
        }} title="Enlace">
          <LinkIcon size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('formatBlock', 'h2'); }} title="Título">
          <Heading2 size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('formatBlock', 'p'); }} title="Párrafo">
          <Type size={16} />
        </button>
      </div>
      <div
        ref={editorRef}
        className={styles.richTextContent}
        contentEditable
        onInput={handleInput}
        onKeyDown={handleKeyDown}
        data-placeholder={placeholder || 'Escribe aquí...'}
      />
    </div>
  )
}
