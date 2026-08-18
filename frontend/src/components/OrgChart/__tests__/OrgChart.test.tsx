import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
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

// jsdom no hace layout: sin medidas, el recorte que impide perder el árbol de
// vista dejaría el desplazamiento clavado en su tope. Se le dan medidas para
// que haya margen donde moverse.
const withLayout = () => {
  const rect = { width: 800, height: 400, top: 0, left: 0, right: 800, bottom: 400, x: 0, y: 0, toJSON: () => ({}) };
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(rect as DOMRect);
  Object.defineProperty(HTMLElement.prototype, 'clientWidth', { value: 1000, configurable: true });
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', { value: 600, configurable: true });
  // jsdom no implementa la captura de puntero.
  HTMLElement.prototype.setPointerCapture = vi.fn();
  HTMLElement.prototype.releasePointerCapture = vi.fn();
};

const canvasOf = (container: HTMLElement) =>
  container.querySelector('[data-org-canvas]') as HTMLElement;

const shiftOf = (el: HTMLElement) => {
  const m = /translate3d\((-?\d+)px, (-?\d+)px/.exec(el.style.transform);
  return m ? { x: Number(m[1]), y: Number(m[2]) } : null;
};

describe('OrgChart — lienzo', () => {
  beforeEach(withLayout);
  afterEach(() => vi.restoreAllMocks());

  it('arrastrar el fondo mueve el árbol', async () => {
    const { container } = render(<OrgChart people={PEOPLE} />);
    const view = canvasOf(container).parentElement!;
    const before = shiftOf(canvasOf(container))!;

    fireEvent.pointerDown(view, { button: 0, clientX: 400, clientY: 300 });
    fireEvent.pointerMove(view, { clientX: 460, clientY: 340 });

    const after = shiftOf(canvasOf(container))!;
    expect(after.x - before.x).toBe(60);
    expect(after.y - before.y).toBe(40);
  });

  // Si agarrar una tarjeta moviera además el lienzo, reasignar un manager
  // arrastraría el árbol entero bajo el puntero.
  it('arrastrar una tarjeta no mueve el lienzo', () => {
    const { container } = render(<OrgChart people={PEOPLE} />);
    const view = canvasOf(container).parentElement!;
    const before = shiftOf(canvasOf(container))!;

    const card = container.querySelector('[data-org-person]') as HTMLElement;
    fireEvent.pointerDown(card, { button: 0, clientX: 400, clientY: 300 });
    fireEvent.pointerMove(view, { clientX: 460, clientY: 340 });

    expect(shiftOf(canvasOf(container))).toEqual(before);
  });

  it('la rueda desplaza el lienzo', () => {
    const { container } = render(<OrgChart people={PEOPLE} />);
    const view = canvasOf(container).parentElement!;
    const before = shiftOf(canvasOf(container))!;

    fireEvent.wheel(view, { deltaX: 30, deltaY: 50 });

    const after = shiftOf(canvasOf(container))!;
    expect(after.x - before.x).toBe(-30);
    expect(after.y - before.y).toBe(-50);
  });
});

describe('OrgChart — zoom', () => {
  const level = () => screen.getByRole('button', { name: /%/ });

  it('aleja y acerca en pasos de 10%', () => {
    render(<OrgChart people={PEOPLE} />);
    expect(level()).toHaveTextContent('100%');

    fireEvent.click(screen.getByRole('button', { name: 'Alejar' }));
    expect(level()).toHaveTextContent('90%');

    fireEvent.click(screen.getByRole('button', { name: 'Acercar' }));
    expect(level()).toHaveTextContent('100%');
  });

  // Sin tope, alejar sin límite deja el árbol ilegible y acercar lo desborda
  // sin necesidad: 30% y 120% son los extremos útiles.
  it('no se pasa de los límites', () => {
    render(<OrgChart people={PEOPLE} />);
    const out = screen.getByRole('button', { name: 'Alejar' });

    for (let i = 0; i < 12; i++) fireEvent.click(out);
    expect(level()).toHaveTextContent('30%');
    expect(out).toBeDisabled();

    const zin = screen.getByRole('button', { name: 'Acercar' });
    for (let i = 0; i < 12; i++) fireEvent.click(zin);
    expect(level()).toHaveTextContent('120%');
    expect(zin).toBeDisabled();
  });

  it('el zoom se aplica al árbol, no a la barra de herramientas', () => {
    const { container } = render(<OrgChart people={PEOPLE} />);
    fireEvent.click(screen.getByRole('button', { name: 'Alejar' }));

    const root = container.querySelector('ul');
    expect(root).toHaveStyle({ zoom: '0.9' });
  });

  // En la vista de lista el zoom no aplica: no hay nada que desbordar.
  it('los controles solo están en la vista de organigrama', () => {
    render(<OrgChart people={PEOPLE} />);
    expect(screen.getByRole('button', { name: 'Alejar' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('tab', { name: /lista/i }));
    expect(screen.queryByRole('button', { name: 'Alejar' })).not.toBeInTheDocument();
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
