import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from './Modal'

describe('Modal', () => {
  it('does not render when closed', () => {
    render(<Modal isOpen={false} onClose={() => {}} title="Hola">contenido</Modal>)
    expect(screen.queryByText('contenido')).toBeNull()
  })

  it('renders title and children when open with dialog semantics', () => {
    render(<Modal isOpen onClose={() => {}} title="Crear">cuerpo</Modal>)
    expect(screen.getByRole('dialog')).toHaveAttribute('aria-modal', 'true')
    expect(screen.getByText('Crear')).toBeInTheDocument()
    expect(screen.getByText('cuerpo')).toBeInTheDocument()
  })

  it('closes on Escape and on backdrop click, but not when clicking the panel', async () => {
    const onClose = vi.fn()
    render(<Modal isOpen onClose={onClose} title="T">x</Modal>)

    await userEvent.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(1)

    await userEvent.click(screen.getByRole('dialog')) // panel: should NOT close
    expect(onClose).toHaveBeenCalledTimes(1)

    // The overlay is the dialog's parent.
    const overlay = screen.getByRole('dialog').parentElement as HTMLElement
    await userEvent.click(overlay)
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  // Regression guard: a parent passing an inline onClose used to re-run the
  // focus-trap effect on every render, stealing focus back to the close button
  // on each keystroke. Focus must stay in the typed input.
  it('keeps focus in an input across re-renders with a fresh onClose each render', async () => {
    function Harness() {
      const [text, setText] = useState('')
      return (
        <Modal isOpen onClose={() => { /* new identity every render */ }} title="Nueva tarea">
          <input aria-label="titulo" autoFocus value={text} onChange={(e) => setText(e.target.value)} />
        </Modal>
      )
    }
    render(<Harness />)
    const input = screen.getByLabelText('titulo') as HTMLInputElement

    await userEvent.type(input, 'Hola')

    expect(input.value).toBe('Hola')
    expect(document.activeElement).toBe(input)
  })
})
