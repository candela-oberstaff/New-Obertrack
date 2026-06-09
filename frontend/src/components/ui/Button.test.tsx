import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button } from './Button'

describe('Button', () => {
  it('renders its label and fires onClick', async () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>Guardar</Button>)
    const btn = screen.getByRole('button', { name: 'Guardar' })
    await userEvent.click(btn)
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('applies the variant and size classes', () => {
    render(<Button variant="danger" size="sm">X</Button>)
    const btn = screen.getByRole('button', { name: 'X' })
    expect(btn.className).toContain('ui-btn--danger')
    expect(btn.className).toContain('ui-btn--sm')
  })

  it('shows a spinner and disables itself while loading', async () => {
    const onClick = vi.fn()
    render(<Button loading onClick={onClick}>Enviar</Button>)
    const btn = screen.getByRole('button')
    expect(btn).toBeDisabled()
    expect(btn).toHaveAttribute('aria-busy', 'true')
    await userEvent.click(btn)
    expect(onClick).not.toHaveBeenCalled()
  })

  it('does not fire onClick when disabled', async () => {
    const onClick = vi.fn()
    render(<Button disabled onClick={onClick}>No</Button>)
    await userEvent.click(screen.getByRole('button'))
    expect(onClick).not.toHaveBeenCalled()
  })
})
