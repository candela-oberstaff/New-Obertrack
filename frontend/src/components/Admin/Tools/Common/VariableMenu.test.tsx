import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { useEmailVariables, VariableMenu } from './VariableMenu';
import { VARIABLE_MIME } from './emailVariables';
import { emailService } from '../../../../services/emailService';

vi.mock('../../../../services/emailService', () => ({
  emailService: { getVariables: vi.fn() },
}));

const VARIABLES = [
  { key: 'nombre', label: 'Nombre completo', description: 'Nombre del destinatario', example: 'María González', fallback: 'colega', group: 'Persona' },
  { key: 'empresa', label: 'Empresa', description: 'Empresa asociada', example: 'Acme Corp', fallback: 'tu empresa', group: 'Empresa' },
];

/** Editor mínimo: un campo enlazado, un bloque como zona de soltado y el menú. */
function Harness({ initial = '' }: { initial?: string }) {
  const [text, setText] = React.useState(initial);
  const { variables, bindField, insertVariable, activeFieldLabel, dragging, dragSourceProps, dropZone } = useEmailVariables();

  return (
    <div>
      <textarea
        aria-label="contenido"
        value={text}
        onChange={e => setText(e.target.value)}
        {...bindField(setText, 'Contenido del texto')}
      />
      <div data-testid="bloque" data-dragging={dragging} {...dropZone(text, setText)}>
        {text}
      </div>
      <VariableMenu
        variables={variables}
        onInsert={insertVariable}
        activeFieldLabel={activeFieldLabel}
        dragSourceProps={dragSourceProps}
      />
    </div>
  );
}

/** DataTransfer no existe en jsdom; este doble guarda y devuelve los formatos. */
function fakeDataTransfer() {
  const store = new Map<string, string>();
  return {
    data: store,
    setData: (type: string, value: string) => { store.set(type, value); },
    getData: (type: string) => store.get(type) ?? '',
    get types() { return [...store.keys()]; },
    effectAllowed: 'none',
    dropEffect: 'none',
    setDragImage: vi.fn(),
  };
}

// El botón solo aparece cuando el catálogo llegó del backend.
const openMenu = async () => {
  fireEvent.click(await screen.findByRole('button', { name: /variables/i }));
  await screen.findByText('{{nombre}}');
};

/**
 * Localiza la fila de una variable por su token. El nombre del grupo puede
 * coincidir con la etiqueta ("Empresa"), el token nunca.
 */
const variableRow = (token: string) =>
  screen.getByText(token).closest('[draggable]') as HTMLElement;

beforeEach(() => {
  vi.mocked(emailService.getVariables).mockResolvedValue(VARIABLES);
});

describe('VariableMenu', () => {
  it('carga el catálogo que sirve el backend', async () => {
    render(<Harness />);
    await openMenu();

    expect(screen.getByText('Nombre completo')).toBeInTheDocument();
    expect(variableRow('{{empresa}}')).toHaveTextContent('Empresa');
    // Se muestra el valor de ejemplo para que se entienda qué escribirá.
    expect(screen.getByText('María González')).toBeInTheDocument();
  });

  it('inserta en el cursor del campo enlazado', async () => {
    render(<Harness initial="Hola , ¿cómo estás?" />);

    const field = screen.getByLabelText('contenido') as HTMLTextAreaElement;
    field.focus();
    field.setSelectionRange(5, 5);
    fireEvent.focus(field);

    await openMenu();
    fireEvent.click(variableRow('{{nombre}}'));

    await waitFor(() => expect(field.value).toBe('Hola {{nombre}}, ¿cómo estás?'));
  });

  // El menú le roba el foco al campo (el buscador se autoenfoca). Si no se
  // anotara el cursor al perderlo, la variable caería al final del texto.
  it('respeta el cursor aunque el campo haya perdido el foco', async () => {
    render(<Harness initial="Hola , ¿cómo estás?" />);

    const field = screen.getByLabelText('contenido') as HTMLTextAreaElement;
    field.focus();
    field.setSelectionRange(5, 5);
    fireEvent.focus(field);

    // Abrir el menú le quita el foco al campo de verdad.
    (await screen.findByRole('button', { name: /variables/i })).focus();
    field.setSelectionRange(0, 0); // el navegador reubica el cursor tras el blur

    await openMenu();
    fireEvent.click(variableRow('{{empresa}}'));

    await waitFor(() => expect(field.value).toBe('Hola {{empresa}}, ¿cómo estás?'));
  });

  // Regresión: un onMouseDown con preventDefault sobre la fila impedía que
  // Chrome arrancase el arrastre nativo.
  it('las filas son arrastrables y publican ambos formatos', async () => {
    render(<Harness />);
    await openMenu();

    const row = variableRow('{{nombre}}');
    expect(row).not.toBeNull();
    expect(row.getAttribute('draggable')).toBe('true');

    const dataTransfer = fakeDataTransfer();
    fireEvent.dragStart(row, { dataTransfer });

    // text/plain habilita el soltado nativo sobre inputs y textareas; el MIME
    // propio es el que aceptan las zonas de soltado del lienzo.
    expect(dataTransfer.getData('text/plain')).toBe('{{nombre}}');
    expect(dataTransfer.getData(VARIABLE_MIME)).toBe('{{nombre}}');
  });

  it('el panel no se desmonta durante el arrastre', async () => {
    render(<Harness />);
    await openMenu();

    const row = variableRow('{{nombre}}');
    fireEvent.dragStart(row, { dataTransfer: fakeDataTransfer() });

    // Quitar el nodo origen del DOM cancela el arrastre en Chrome: debe seguir
    // presente, solo apartado.
    expect(screen.getByText('Nombre completo')).toBeInTheDocument();
    expect(screen.getByTestId('bloque').dataset.dragging).toBe('true');
  });

  // Regresión: un arrastre cancelado disparaba dragEnd y cerraba el panel, lo
  // que se percibía como "hago clic en la variable y se cierra el menú".
  it('un arrastre cancelado deja el panel abierto', async () => {
    render(<Harness />);
    await openMenu();

    const row = variableRow('{{nombre}}');
    const dataTransfer = fakeDataTransfer();
    fireEvent.dragStart(row, { dataTransfer });
    fireEvent.dragEnd(row, { dataTransfer: { ...dataTransfer, dropEffect: 'none' } });

    expect(screen.getByText('Nombre completo')).toBeInTheDocument();
  });

  it('el panel se cierra tras un soltado real', async () => {
    render(<Harness initial="Hola" />);
    await openMenu();

    const row = variableRow('{{nombre}}');
    const dataTransfer = fakeDataTransfer();
    fireEvent.dragStart(row, { dataTransfer });
    fireEvent.dragEnd(row, { dataTransfer: { ...dataTransfer, dropEffect: 'copy' } });

    await waitFor(() => expect(screen.queryByText('Nombre completo')).not.toBeInTheDocument());
  });

  // Sin campo elegido el clic no hacía nada y parecía que el menú fallaba.
  it('avisa cuando no hay dónde insertar', async () => {
    render(<Harness />);
    await openMenu();

    fireEvent.click(variableRow('{{nombre}}'));

    expect(await screen.findByText(/Primero haz clic en el texto/i)).toBeInTheDocument();
  });

  it('soltar sobre un bloque inserta la variable', async () => {
    render(<Harness initial="Hola mundo" />);
    await openMenu();

    const row = variableRow('{{nombre}}');
    const dataTransfer = fakeDataTransfer();
    fireEvent.dragStart(row, { dataTransfer });

    const bloque = screen.getByTestId('bloque');
    fireEvent.dragOver(bloque, { dataTransfer });
    fireEvent.drop(bloque, { dataTransfer, clientX: 10, clientY: 10 });

    // Sin caretRangeFromPoint en jsdom, la inserción cae al final del texto.
    await waitFor(() => expect(screen.getByLabelText('contenido')).toHaveValue('Hola mundo{{nombre}}'));
  });

  it('ignora lo que se arrastre sin el formato propio', async () => {
    render(<Harness initial="Hola mundo" />);

    const foreign = fakeDataTransfer();
    foreign.setData('text/plain', 'texto de otra parte');

    const bloque = screen.getByTestId('bloque');
    fireEvent.drop(bloque, { dataTransfer: foreign, clientX: 10, clientY: 10 });

    expect(screen.getByLabelText('contenido')).toHaveValue('Hola mundo');
  });
});
