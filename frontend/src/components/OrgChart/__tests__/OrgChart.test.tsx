import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { OrgChart } from '../OrgChart';
import type { OrgPerson } from '../orgTree';

const person = (over: Partial<OrgPerson> & { user_id: number; name: string }): OrgPerson => ({
  employment_id: over.user_id,
  email: `${over.user_id}@x.com`,
  is_manager: false,
  is_active: true,
  manager_id: null,
  ...over,
});

const PEOPLE: OrgPerson[] = [
  person({ user_id: 1, name: 'Acme S.A', is_company: true }),
  person({ user_id: 2, name: 'Ana Rivas', manager_id: 1, is_manager: true, job_title: 'Backend Developer' }),
  person({ user_id: 3, name: 'Beto Salas', manager_id: 2, job_title: 'QA' }),
];

const href = (p: OrgPerson) => `/admin/users/${p.user_id}`;

describe('OrgChart — enlace al perfil', () => {
  it('cada persona enlaza a su ficha en una pestaña nueva', () => {
    render(<OrgChart people={PEOPLE} profileHref={href} />);

    const ana = screen.getByRole('link', { name: 'Ana Rivas' });
    expect(ana).toHaveAttribute('href', '/admin/users/2');
    expect(ana).toHaveAttribute('target', '_blank');
    // Sin esto la pestaña nueva puede manipular a la que la abrió.
    expect(ana).toHaveAttribute('rel', expect.stringContaining('noopener'));
  });

  // La cuenta de empresa es la cabeza del árbol, no una persona: no hay ficha
  // que abrir y enlazarla llevaría a una pantalla inexistente.
  it('la cuenta de empresa no se enlaza', () => {
    render(<OrgChart people={PEOPLE} profileHref={href} />);

    expect(screen.queryByRole('link', { name: 'Acme S.A' })).not.toBeInTheDocument();
    expect(screen.getByText('Acme S.A')).toBeInTheDocument();
  });

  // Un supervisor no tiene acceso a ninguna pantalla de perfil ajeno: sin
  // profileHref las tarjetas se quedan sin enlace en vez de llevar a un error.
  it('sin profileHref no se enlaza a nadie', () => {
    render(<OrgChart people={PEOPLE} />);

    expect(screen.queryAllByRole('link')).toHaveLength(0);
    expect(screen.getByText('Ana Rivas')).toBeInTheDocument();
  });

  it('el nombre no abre dos pestañas: el clic no se propaga a la tarjeta', () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null);
    render(<OrgChart people={PEOPLE} profileHref={href} />);

    fireEvent.click(screen.getByRole('link', { name: 'Beto Salas' }));

    // El enlace lo resuelve el navegador; la tarjeta no debe abrir otra encima.
    expect(open).not.toHaveBeenCalled();
    open.mockRestore();
  });

  it('un clic en la tarjeta abre la ficha en una pestaña nueva', () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null);
    render(<OrgChart people={PEOPLE} profileHref={href} />);

    // El cargo es parte de la tarjeta pero no del enlace del nombre.
    fireEvent.click(screen.getByText('Backend Developer'));

    expect(open).toHaveBeenCalledWith('/admin/users/2', '_blank', 'noopener,noreferrer');
    open.mockRestore();
  });

  it('la tarjeta de la empresa no abre nada al clicarla', () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null);
    render(<OrgChart people={PEOPLE} profileHref={href} />);

    fireEvent.click(screen.getByText('Acme S.A'));

    expect(open).not.toHaveBeenCalled();
    open.mockRestore();
  });
});

describe('OrgChart — vista ampliada', () => {
  it('se puede ampliar y volver, y Escape también cierra', () => {
    render(<OrgChart people={PEOPLE} profileHref={href} />);

    const btn = () => screen.getByRole('button', { name: /ampliar|reducir/i });
    expect(btn()).toHaveAttribute('aria-pressed', 'false');

    fireEvent.click(btn());
    expect(btn()).toHaveAttribute('aria-pressed', 'true');

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(btn()).toHaveAttribute('aria-pressed', 'false');
  });

  // Con el organigrama a pantalla completa, el fondo no debe seguir moviéndose
  // por detrás; al cerrar hay que devolver el scroll o la página queda trabada.
  it('bloquea el desplazamiento del fondo solo mientras está ampliado', () => {
    render(<OrgChart people={PEOPLE} />);
    const btn = () => screen.getByRole('button', { name: /ampliar|reducir/i });

    fireEvent.click(btn());
    expect(document.body.style.overflow).toBe('hidden');

    fireEvent.click(btn());
    expect(document.body.style.overflow).not.toBe('hidden');
  });
});
