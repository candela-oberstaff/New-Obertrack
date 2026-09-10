import { describe, it, expect } from 'vitest'
import { previewKind } from './FilePreviewModal'

describe('previewKind · qué archivo se puede ver sin descargarlo', () => {
  it('reconoce el tipo por el mime que guardamos', () => {
    expect(previewKind({ id: 1, file_name: 'a.png', mime_type: 'image/png' })).toBe('image')
    expect(previewKind({ id: 1, file_name: 'c.pdf', mime_type: 'application/pdf' })).toBe('pdf')
    expect(previewKind({ id: 1, file_name: 'v.mp4', mime_type: 'video/mp4' })).toBe('video')
    expect(previewKind({ id: 1, file_name: 'a.mp3', mime_type: 'audio/mpeg' })).toBe('audio')
  })

  it('cae a la extensión cuando no hay mime', () => {
    // Los documentos subidos antes de que se registrara el mime siguen en el
    // expediente y merecen verse igual: sin este respaldo, todos caerían en
    // "no se puede previsualizar".
    expect(previewKind({ id: 1, file_name: 'Captura 2026-08-29.PNG' })).toBe('image')
    expect(previewKind({ id: 1, file_name: 'contrato.pdf', mime_type: '' })).toBe('pdf')
  })

  it('un nombre con puntos no confunde la extensión', () => {
    expect(previewKind({ id: 1, file_name: '1_1788200964_Captura de pantalla 2026-08-29 144453png.png' })).toBe('image')
  })

  it('lo que no se puede mostrar se declara como tal en vez de intentarlo', () => {
    // Enseñar un .docx en un iframe da una descarga o una página en blanco;
    // decirlo claro y ofrecer el botón de bajarlo es más honesto.
    expect(previewKind({ id: 1, file_name: 'informe.docx' })).toBe('none')
    expect(previewKind({ id: 1, file_name: 'backup.zip', mime_type: 'application/zip' })).toBe('none')
    expect(previewKind({ id: 1, file_name: 'sinextension' })).toBe('none')
  })
})
