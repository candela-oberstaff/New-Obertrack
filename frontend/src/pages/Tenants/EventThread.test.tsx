import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { EventThread } from './EventThread'

vi.mock('../../components/ui/ConfirmProvider', () => ({
  useConfirm: () => vi.fn().mockResolvedValue(true),
}))

vi.mock('../../context/NotificationContext', () => ({
  useNotification: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }),
}))

const addComment = vi.fn()
const addAttachment = vi.fn()

const props = () => ({
  tenantId: 7,
  eventId: 50,
  canEdit: true,
  addComment,
  updateComment: vi.fn().mockResolvedValue(undefined),
  deleteComment: vi.fn().mockResolvedValue(undefined),
  addAttachment,
  deleteAttachment: vi.fn().mockResolvedValue(undefined),
})

const file = (name: string, type = 'application/pdf') =>
  new File(['x'], name, { type })

const attach = (f: File) => {
  const input = document.querySelector('input[type="file"]') as HTMLInputElement
  fireEvent.change(input, { target: { files: [f] } })
}

beforeEach(() => {
  vi.clearAllMocks()
  addComment.mockResolvedValue(999)
  addAttachment.mockResolvedValue(undefined)
})

describe('EventThread', () => {
  // Lo que motivó todo esto: adjuntar mientras se escribe subía el archivo al
  // instante y lo dejaba suelto en la entrada, mientras el texto se publicaba
  // por separado. Un mensaje y su captura son una sola cosa.
  it('el archivo adjuntado al escribir viaja CON el comentario', async () => {
    render(<EventThread {...props()} />)
    fireEvent.click(screen.getByRole('button', { name: /comentar o adjuntar/i }))

    const f = file('captura.png', 'image/png')
    attach(f)
    fireEvent.change(screen.getByLabelText(/nuevo comentario/i), {
      target: { value: 'Mira este error' },
    })

    // Mientras no se envía, nada ha subido todavía.
    expect(addAttachment).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: /^comentar$/i }))

    await waitFor(() => expect(addComment).toHaveBeenCalledWith(50, 'Mira este error'))
    // El id del comentario recién creado es lo que ata el archivo al mensaje.
    await waitFor(() => expect(addAttachment).toHaveBeenCalledWith(50, f, 999))
  })

  it('sin texto, los archivos se cuelgan de la entrada', async () => {
    render(<EventThread {...props()} />)
    fireEvent.click(screen.getByRole('button', { name: /comentar o adjuntar/i }))

    const f = file('contrato.pdf')
    attach(f)

    // La etiqueta del botón dice qué va a pasar, en vez de dejarlo a la
    // adivinación.
    fireEvent.click(await screen.findByRole('button', { name: /adjuntar a la entrada/i }))

    await waitFor(() => expect(addAttachment).toHaveBeenCalledWith(50, f))
    expect(addComment).not.toHaveBeenCalled()
  })

  it('los archivos en cola se pueden quitar antes de enviar', async () => {
    render(<EventThread {...props()} />)
    fireEvent.click(screen.getByRole('button', { name: /comentar o adjuntar/i }))

    attach(file('sobra.pdf'))
    fireEvent.click(await screen.findByRole('button', { name: /quitar sobra\.pdf/i }))

    await waitFor(() =>
      expect(screen.queryByText('sobra.pdf')).not.toBeInTheDocument()
    )
    // Sin texto ni archivos no hay nada que enviar.
    expect(screen.getByRole('button', { name: /^comentar$/i })).toBeDisabled()
  })

  // Si falla una subida, volver a buscar los archivos en el disco sería la peor
  // forma de recuperarse de un error de red.
  it('si falla el envío la cola se conserva', async () => {
    addAttachment.mockRejectedValueOnce(new Error('red caída'))
    render(<EventThread {...props()} />)
    fireEvent.click(screen.getByRole('button', { name: /comentar o adjuntar/i }))

    attach(file('captura.png', 'image/png'))
    fireEvent.change(screen.getByLabelText(/nuevo comentario/i), { target: { value: 'texto' } })
    fireEvent.click(screen.getByRole('button', { name: /^comentar$/i }))

    await waitFor(() => expect(addAttachment).toHaveBeenCalled())
    expect(await screen.findByText('captura.png')).toBeInTheDocument()
  })

  it('separa los archivos de la entrada de los de cada comentario', () => {
    const thread = {
      comments: [
        {
          id: 1, event_id: 50, content: 'Con captura', author: 'Ana',
          created_at: '2026-07-31T10:00:00Z',
          attachments: [{ id: 2, event_id: 50, comment_id: 1, file_name: 'en-comentario.png', file_size: 10, mime_type: 'image/png', author: 'Ana', created_at: '2026-07-31T10:00:00Z' }],
        },
      ],
      attachments: [
        { id: 3, event_id: 50, file_name: 'en-la-entrada.pdf', file_size: 20, mime_type: 'application/pdf', author: 'Ana', created_at: '2026-07-31T10:00:00Z' },
      ],
    }
    render(<EventThread {...props()} thread={thread} />)

    // Los sueltos van bajo su propio rótulo para que se entienda por qué no
    // pertenecen a ningún mensaje.
    expect(screen.getByText(/archivos de esta entrada/i)).toBeInTheDocument()
    expect(screen.getByText('en-la-entrada.pdf')).toBeInTheDocument()
    expect(screen.getByText('en-comentario.png')).toBeInTheDocument()
  })
})
