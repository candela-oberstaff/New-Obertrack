import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { TestSendButton } from './TestSendButton';
import { emailService } from '../../../../services/emailService';

vi.mock('../../../../services/emailService', () => ({
  emailService: {
    getAvailableRecipients: vi.fn(),
    sendTestEmail: vi.fn(),
  },
}));

vi.mock('../../../../context/AuthContext', () => ({
  useAuth: () => ({ user: { id: 1, name: 'Yo', email: 'yo@obertrack.com' } }),
}));

const PEOPLE = [
  { id: 10, name: 'Adriana Carolina Gómez Rondón' },
  { id: 11, name: 'Alejandra Zerpa Urbina' },
];

const payload = () => ({ subject: 'Hola {{nombre}}', blocks: '[{"type":"text"}]' });

const openPanel = async () => {
  fireEvent.click(screen.getByRole('button', { name: /enviar prueba/i }));
  await screen.findByText('yo@obertrack.com');
};

beforeEach(() => {
  // Sin esto las llamadas se acumulan entre casos y "no fue llamado" nunca pasa.
  vi.clearAllMocks();
  // La dirección de prueba se recuerda entre sesiones: sin limpiarla, un caso
  // arrastraría la que dejó el anterior.
  localStorage.clear();
  vi.mocked(emailService.getAvailableRecipients).mockResolvedValue(PEOPLE as never);
  vi.mocked(emailService.sendTestEmail).mockResolvedValue({ to: 'yo@obertrack.com', viewed_as: 'datos de ejemplo' });
});

const editTo = async (value: string) => {
  fireEvent.click(screen.getByRole('button', { name: /cambiar la dirección/i }));
  const input = await screen.findByLabelText(/dirección donde llegará la prueba/i);
  fireEvent.change(input, { target: { value } });
  fireEvent.click(screen.getByRole('button', { name: /^usar$/i }));
  return input;
};

describe('TestSendButton', () => {
  it('por defecto la prueba va al correo propio', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    expect(screen.getByText('yo@obertrack.com')).toBeInTheDocument();
    // Sin dirección propia elegida no se ofrece "volver a mi correo": sería un
    // botón que no hace nada.
    expect(screen.queryByRole('button', { name: /volver a mi propio correo/i })).not.toBeInTheDocument();
  });

  it('permite cambiar la dirección y la envía a esa', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();
    await editTo('marketing@empresa.com');

    expect(await screen.findByText('marketing@empresa.com')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /enviar la prueba/i }));

    await waitFor(() => expect(emailService.sendTestEmail).toHaveBeenCalledWith({
      subject: 'Hola {{nombre}}',
      blocks: '[{"type":"text"}]',
      as_user_id: undefined,
      to_email: 'marketing@empresa.com',
    }));
  });

  it('rechaza una dirección que no lo parece y no la guarda', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();
    await editTo('esto-no-es-un-correo');

    expect(await screen.findByText(/no parece válida/i)).toBeInTheDocument();
    // Sigue en edición: no se traga el valor malo ni cierra como si valiera.
    expect(screen.getByLabelText(/dirección donde llegará la prueba/i)).toBeInTheDocument();
    expect(localStorage.getItem('obertrack.testSend.lastTo')).toBeNull();
  });

  // Quien revisa plantillas lo hace muchas veces seguidas: volver a teclear la
  // dirección corporativa cada vez es la fricción que se venía a quitar.
  it('recuerda la última dirección usada', async () => {
    const { unmount } = render(<TestSendButton getPayload={payload} />);
    await openPanel();
    await editTo('marketing@empresa.com');
    await screen.findByText('marketing@empresa.com');
    unmount();

    render(<TestSendButton getPayload={payload} />);
    fireEvent.click(screen.getByRole('button', { name: /enviar prueba/i }));

    expect(await screen.findByText('marketing@empresa.com')).toBeInTheDocument();
  });

  it('se puede volver al correo propio', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();
    await editTo('marketing@empresa.com');
    await screen.findByText('marketing@empresa.com');

    fireEvent.click(screen.getByRole('button', { name: /volver a mi propio correo/i }));

    expect(await screen.findByText('yo@obertrack.com')).toBeInTheDocument();
    expect(localStorage.getItem('obertrack.testSend.lastTo')).toBeNull();
  });

  // Mandar a una dirección ajena es fácil de hacer sin querer al cambiar de
  // plantilla, así que la ficha lo dice en vez de dejarlo en un campo discreto.
  it('avisa cuando el destino no es el correo propio', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();
    await editTo('marketing@empresa.com');

    expect(await screen.findByText(/no es tu correo/i)).toBeInTheDocument();
  });

  // Regresión: el menú del Select se renderiza en un portal fuera del panel, y
  // el detector de "clic fuera" lo tomaba como salida. Se cerraba todo y no se
  // seleccionaba a nadie.
  it('elegir una persona no cierra el panel', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    fireEvent.click(await screen.findByRole('button', { name: /persona cuyos datos/i }));
    const option = await screen.findByText('Alejandra Zerpa Urbina');

    // El menú vive en un portal: hay que simular el mousedown que dispara el
    // cierre, no solo el click.
    fireEvent.mouseDown(option);
    fireEvent.click(option);

    expect(screen.getByText('yo@obertrack.com')).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /persona cuyos datos/i })).toHaveTextContent('Alejandra Zerpa Urbina')
    );
  });

  it('envía la prueba con los datos de la persona elegida', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    fireEvent.click(await screen.findByRole('button', { name: /persona cuyos datos/i }));
    const option = await screen.findByText('Adriana Carolina Gómez Rondón');
    fireEvent.mouseDown(option);
    fireEvent.click(option);

    fireEvent.click(screen.getByRole('button', { name: /enviarme la prueba/i }));

    await waitFor(() => expect(emailService.sendTestEmail).toHaveBeenCalledWith({
      subject: 'Hola {{nombre}}',
      blocks: '[{"type":"text"}]',
      as_user_id: 10,
      to_email: undefined,
    }));
  });

  it('sin elegir a nadie usa los datos de ejemplo', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    fireEvent.click(screen.getByRole('button', { name: /enviarme la prueba/i }));

    await waitFor(() => expect(emailService.sendTestEmail).toHaveBeenCalledWith({
      subject: 'Hola {{nombre}}',
      blocks: '[{"type":"text"}]',
      as_user_id: undefined,
      to_email: undefined,
    }));
  });

  it('avisa cuando todavía no hay nada que probar', async () => {
    render(<TestSendButton getPayload={() => null} />);
    await openPanel();

    fireEvent.click(screen.getByRole('button', { name: /enviarme la prueba/i }));

    expect(await screen.findByText(/no hay contenido que probar/i)).toBeInTheDocument();
    expect(emailService.sendTestEmail).not.toHaveBeenCalled();
  });

  it('un clic realmente fuera sí cierra el panel', async () => {
    render(<TestSendButton getPayload={payload} />);
    await openPanel();

    fireEvent.mouseDown(document.body);

    await waitFor(() => expect(screen.queryByText('yo@obertrack.com')).not.toBeInTheDocument());
  });
});
