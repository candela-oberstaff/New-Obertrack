import React, { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Save, Send, Plus, Trash2, GripVertical, Settings, Star, X, Search, Check } from 'lucide-react';
import styles from './SurveyBuilder.module.css';
import commonStyles from '../Tools.module.css';
import { Select } from '../../../ui/Select';
import { SurveyQuestion } from '../../../../services/surveyService';
import { userService } from '../../../../services/user.service';

interface SurveyBuilderProps {
  onBack: () => void;
  onSave: (data: { id?: number; title: string; description: string; questions: SurveyQuestion[]; send_by_email: boolean; send_by_inapp: boolean; recipientIds: number[] }) => Promise<void>;
  onSend?: (data: { id?: number; title: string; description: string; questions: SurveyQuestion[]; send_by_email: boolean; send_by_inapp: boolean; recipientIds: number[] }) => Promise<void>;
  initialData?: {
    id?: number;
    title: string;
    description: string;
    questions: SurveyQuestion[];
    send_by_email: boolean;
    send_by_inapp: boolean;
    recipient_list?: string;
  };
}

const SurveyBuilder: React.FC<SurveyBuilderProps> = ({ onBack, onSave, onSend, initialData }) => {
  const [title, setTitle] = useState(initialData?.title || 'Nueva Encuesta');
  const [description, setDescription] = useState(initialData?.description || '');
  const [questions, setQuestions] = useState<SurveyQuestion[]>(initialData?.questions || []);
  const [showSettings, setShowSettings] = useState(false);
  const [sendByEmail, setSendByEmail] = useState(initialData?.send_by_email ?? true);
  const [sendByInApp, setSendByInApp] = useState(initialData?.send_by_inapp ?? true);
  
  const [selectedRecipients, setSelectedRecipients] = useState<number[]>([]);
  const [userSearch, setUserSearch] = useState('');
  const [isSending, setIsSending] = useState(false);

  const { data: availableUsers = [] } = useQuery({
    queryKey: ['survey-recipients'],
    queryFn: async () => (await userService.getAll({ limit: 1000 })).data || [],
  });

  useEffect(() => {
    if (initialData?.recipient_list) {
      try {
        const ids = JSON.parse(initialData.recipient_list);
        if (Array.isArray(ids)) setSelectedRecipients(ids);
      } catch (e) {
        console.error("Error parsing recipient list", e);
      }
    }
  }, [initialData]);

  const toggleRecipient = (userId: number) => {
    if (selectedRecipients.includes(userId)) {
      setSelectedRecipients(selectedRecipients.filter(id => id !== userId));
    } else {
      setSelectedRecipients([...selectedRecipients, userId]);
    }
  };

  const selectAll = () => {
    setSelectedRecipients(availableUsers.map(u => u.id));
  };

  const selectNone = () => {
    setSelectedRecipients([]);
  };

  const filteredUsers = availableUsers.filter(u => 
    u.name.toLowerCase().includes(userSearch.toLowerCase()) || 
    u.email.toLowerCase().includes(userSearch.toLowerCase())
  );

  const addQuestion = (type: 'text' | 'rating' | 'choice' | 'checkbox' | 'dropdown' | 'linear_scale' | 'grid' | 'checkbox_grid') => {
    let defaultOptions: string | undefined = undefined;
    if (type === 'choice' || type === 'checkbox' || type === 'dropdown') {
      defaultOptions = JSON.stringify(['Opción 1']);
    } else if (type === 'linear_scale') {
      defaultOptions = JSON.stringify({ min: 1, max: 5, minLabel: 'Malo', maxLabel: 'Excelente' });
    } else if (type === 'grid' || type === 'checkbox_grid') {
      defaultOptions = JSON.stringify({ rows: ['Fila 1'], columns: ['Columna 1'] });
    }

    setQuestions([...questions, {
      text: '',
      type,
      is_required: true,
      order_index: questions.length,
      options: defaultOptions
    }]);
  };

  const updateQuestion = (index: number, field: keyof SurveyQuestion, value: any) => {
    const newQuestions = [...questions];
    newQuestions[index] = { ...newQuestions[index], [field]: value };
    setQuestions(newQuestions);
  };

  const removeQuestion = (index: number) => {
    const newQuestions = questions.filter((_, i) => i !== index);
    newQuestions.forEach((q, i) => q.order_index = i);
    setQuestions(newQuestions);
  };

  const [draggedItemIndex, setDraggedItemIndex] = useState<number | null>(null);

  const handleDragStart = (e: React.DragEvent, index: number) => {
    setDraggedItemIndex(index);
    e.dataTransfer.effectAllowed = "move";
  };

  const handleDragEnter = (index: number) => {
    if (draggedItemIndex === null || draggedItemIndex === index) return;
    const newQuestions = [...questions];
    const draggedItemContent = newQuestions[draggedItemIndex];
    newQuestions.splice(draggedItemIndex, 1);
    newQuestions.splice(index, 0, draggedItemContent);
    newQuestions.forEach((q, i) => q.order_index = i);
    setDraggedItemIndex(index);
    setQuestions(newQuestions);
  };

  const handleDragEnd = () => {
    setDraggedItemIndex(null);
  };

  const addChoiceOption = (questionIndex: number) => {
    const newQuestions = [...questions];
    const q = newQuestions[questionIndex];
    const opts = JSON.parse(q.options || '[]');
    opts.push(`Opción ${opts.length + 1}`);
    q.options = JSON.stringify(opts);
    setQuestions(newQuestions);
  };

  const updateChoiceOption = (questionIndex: number, optionIndex: number, value: string) => {
    const newQuestions = [...questions];
    const q = newQuestions[questionIndex];
    const opts = JSON.parse(q.options || '[]');
    opts[optionIndex] = value;
    q.options = JSON.stringify(opts);
    setQuestions(newQuestions);
  };

  const removeChoiceOption = (questionIndex: number, optionIndex: number) => {
    const newQuestions = [...questions];
    const q = newQuestions[questionIndex];
    const opts = JSON.parse(q.options || '[]');
    opts.splice(optionIndex, 1);
    q.options = JSON.stringify(opts);
    setQuestions(newQuestions);
  };

  const handleSave = async () => {
    if (isSending) return;
    setIsSending(true);
    try {
      await onSave({ 
        id: initialData?.id, 
        title, 
        description, 
        questions, 
        send_by_email: sendByEmail, 
        send_by_inapp: sendByInApp, 
        recipientIds: selectedRecipients 
      });
    } finally {
      setIsSending(false);
    }
  };

  const handleSend = async () => {
    if (isSending) return;
    if (selectedRecipients.length === 0) {
      alert("Por favor, selecciona al menos un destinatario en 'Configuración' antes de enviar la encuesta.");
      return;
    }
    setIsSending(true);
    try {
      if (onSend) {
        await onSend({
          id: initialData?.id,
          title,
          description,
          questions,
          send_by_email: sendByEmail,
          send_by_inapp: sendByInApp,
          recipientIds: selectedRecipients
        });
      }
    } finally {
      setIsSending(false);
    }
  };

  return (
    <div className={styles.builderContainer}>
      <header className={styles.builderHeader}>
        <div className={styles.headerLeft}>
          <button className={commonStyles['btn-outline']} onClick={onBack} disabled={isSending}>
            <ArrowLeft size={16} /> Volver
          </button>
          <input
            type="text"
            className={styles.titleInput}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Título de la encuesta"
            disabled={isSending}
          />
        </div>
        <div className={styles.headerActions}>
          <button className={commonStyles['btn-outline']} onClick={() => setShowSettings(true)} disabled={isSending}>
            <Settings size={16} /> Configuración
          </button>
          <button className={commonStyles['btn-outline']} onClick={handleSave} disabled={isSending}>
            {isSending ? <div className={styles.loader}></div> : <><Save size={16} /> Guardar Borrador</>}
          </button>
          <button className={commonStyles['btn-primary']} onClick={handleSend} disabled={isSending}>
            {isSending ? <div className={styles.loader}></div> : <><Send size={16} /> Continuar y Enviar</>}
          </button>
        </div>
        <div className={styles.mobileActions}>
          <Select
            fullWidth
            disabled={isSending}
            value=""
            placeholder="Acciones..."
            onChange={(val) => {
              if (val === 'settings') setShowSettings(true);
              if (val === 'save') handleSave();
              if (val === 'send') handleSend();
            }}
            options={[
              { value: 'settings', label: 'Configuración' },
              { value: 'save', label: 'Guardar Borrador' },
              { value: 'send', label: 'Continuar y Enviar' },
            ]}
          />
        </div>
      </header>

      <div className={styles.builderContent}>
        {showSettings && (
          <div className={styles.modalOverlay}>
            <div className={styles.settingsModal}>
              <div className={styles.modalHeader}>
                <h3>Configuración de Encuesta</h3>
                <button className={styles.closeModal} onClick={() => setShowSettings(false)}>
                  <X size={20} />
                </button>
              </div>
              
              <div className={styles.modalBody}>
                <section className={styles.settingsSection}>
                  <h4>Métodos de Envío</h4>
                  <div className={styles.methodsGrid}>
                    <label className={`${styles.methodCard} ${sendByInApp ? styles.active : ''}`}>
                      <input type="checkbox" checked={sendByInApp} onChange={e => setSendByInApp(e.target.checked)} />
                      <div className={styles.methodIcon}><Plus size={20} /></div>
                      <span>Notificación Platforma</span>
                    </label>
                    <label className={`${styles.methodCard} ${sendByEmail ? styles.active : ''}`}>
                      <input type="checkbox" checked={sendByEmail} onChange={e => setSendByEmail(e.target.checked)} />
                      <div className={styles.methodIcon}><Send size={20} /></div>
                      <span>Email Marketing</span>
                    </label>
                  </div>
                </section>

                <section className={styles.settingsSection}>
                  <div className={styles.sectionHeaderFlex}>
                    <h4>Destinatarios ({selectedRecipients.length})</h4>
                    <div className={styles.selectionActions}>
                      <button onClick={selectAll}>Todos</button>
                      <button onClick={selectNone}>Ninguno</button>
                    </div>
                  </div>

                  <div className={styles.searchContainer}>
                    <Search size={16} />
                    <input 
                      type="text" 
                      placeholder="Buscar usuarios..." 
                      value={userSearch}
                      onChange={e => setUserSearch(e.target.value)}
                    />
                  </div>

                  <div className={styles.userList}>
                    {filteredUsers.map(user => (
                      <div 
                        key={user.id} 
                        className={`${styles.userItem} ${selectedRecipients.includes(user.id) ? styles.selected : ''}`}
                        onClick={() => toggleRecipient(user.id)}
                      >
                        <div className={styles.userAvatar}>
                          {user.name.charAt(0)}
                        </div>
                        <div className={styles.userInfo}>
                          <span className={styles.userName}>{user.name}</span>
                          <span className={styles.userEmail}>{user.email}</span>
                        </div>
                        <div className={styles.checkboxMock}>
                          {selectedRecipients.includes(user.id) && <Check size={14} />}
                        </div>
                      </div>
                    ))}
                  </div>
                </section>
              </div>

              <div className={styles.modalFooter}>
                <button className={commonStyles['btn-primary']} onClick={() => setShowSettings(false)}>
                  Guardar Configuración
                </button>
              </div>
            </div>
          </div>
        )}

        <div className={styles.canvasArea}>
          <div className={styles.surveyForm}>
            <textarea
              className={styles.descriptionInput}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Escribe una breve descripción o instrucciones para la encuesta..."
            />

            <div className={styles.questionsList}>
              {questions.map((q, index) => (
                <div 
                  key={index} 
                  className={`${styles.questionCard} ${draggedItemIndex === index ? styles.dragging : ''}`}
                  draggable
                  onDragStart={(e) => handleDragStart(e, index)}
                  onDragEnter={() => handleDragEnter(index)}
                  onDragEnd={handleDragEnd}
                  onDragOver={(e) => e.preventDefault()}
                >
                  <div className={styles.questionHeader}>
                    <div className={styles.dragHandle}>
                      <GripVertical size={16} />
                    </div>
                    <input
                      type="text"
                      className={styles.questionTextInput}
                      value={q.text}
                      onChange={(e) => updateQuestion(index, 'text', e.target.value)}
                      placeholder="Escribe tu pregunta aquí..."
                    />
                    <button className={styles.deleteBtn} onClick={() => removeQuestion(index)}>
                      <Trash2 size={16} />
                    </button>
                  </div>

                  <div className={styles.questionBody}>
                    {q.type === 'text' && (
                      <div className={styles.mockInput}>El usuario verá una caja de texto libre.</div>
                    )}
                    {q.type === 'rating' && (
                      <div className={styles.mockRating}>
                        {[1, 2, 3, 4, 5].map(star => (
                          <Star key={star} size={28} color="#cbd5e1" strokeWidth={1.5} />
                        ))}
                      </div>
                    )}
                    {(q.type === 'choice' || q.type === 'checkbox' || q.type === 'dropdown') && (
                      <div className={styles.choiceEditor}>
                        {q.type === 'dropdown' && <div className={styles.mockInput} style={{marginBottom: 8}}>↓ El usuario verá un menú desplegable</div>}
                        {(JSON.parse(q.options || '[]') as string[]).map((opt, oIndex) => (
                          <div key={oIndex} className={styles.choiceOptionEdit}>
                            <div className={q.type === 'checkbox' ? styles.choiceCheckMock : styles.choiceRadioMock}></div>
                            <input
                              type="text"
                              value={opt}
                              onChange={(e) => updateChoiceOption(index, oIndex, e.target.value)}
                              className={styles.choiceInputEdit}
                              placeholder="Escribe la opción..."
                            />
                            <button className={styles.removeOptionBtn} onClick={() => removeChoiceOption(index, oIndex)}>
                              <X size={14} />
                            </button>
                          </div>
                        ))}
                        <button className={styles.addOptionBtn} onClick={() => addChoiceOption(index)}>
                          <Plus size={14} /> Añadir opción
                        </button>
                      </div>
                    )}
                    {q.type === 'linear_scale' && (() => {
                      let cfg: any = {};
                      try { cfg = JSON.parse(q.options || '{}'); } catch {}
                      return (
                        <div className={styles.linearScaleEditor}>
                          <div className={styles.scaleRangeRow}>
                            <div className={styles.scaleField}>
                              <label>Mín</label>
                              <input type="number" value={cfg.min ?? 1} min={0} max={9}
                                onChange={e => { cfg.min = parseInt(e.target.value); updateQuestion(index, 'options', JSON.stringify(cfg)); }} />
                            </div>
                            <div className={styles.scaleMockTrack}>
                              {Array.from({length: (cfg.max ?? 5) - (cfg.min ?? 1) + 1}, (_, i) => (cfg.min ?? 1) + i).map(n => (
                                <span key={n} className={styles.scaleDot}>{n}</span>
                              ))}
                            </div>
                            <div className={styles.scaleField}>
                              <label>Máx</label>
                              <input type="number" value={cfg.max ?? 5} min={2} max={10}
                                onChange={e => { cfg.max = parseInt(e.target.value); updateQuestion(index, 'options', JSON.stringify(cfg)); }} />
                            </div>
                          </div>
                          <div className={styles.scaleLabelsRow}>
                            <input type="text" className={styles.scaleLabelInput} value={cfg.minLabel ?? ''} placeholder="Etiqueta mínimo"
                              onChange={e => { cfg.minLabel = e.target.value; updateQuestion(index, 'options', JSON.stringify(cfg)); }} />
                            <input type="text" className={styles.scaleLabelInput} value={cfg.maxLabel ?? ''} placeholder="Etiqueta máximo"
                              onChange={e => { cfg.maxLabel = e.target.value; updateQuestion(index, 'options', JSON.stringify(cfg)); }} />
                          </div>
                        </div>
                      );
                    })()}
                    {(q.type === 'grid' || q.type === 'checkbox_grid') && (() => {
                      let cfg: any = { rows: ['Fila 1'], columns: ['Columna 1'] };
                      try { cfg = JSON.parse(q.options || '{}'); } catch {}
                      const updateGridField = (field: 'rows' | 'columns', vals: string[]) => {
                        updateQuestion(index, 'options', JSON.stringify({ ...cfg, [field]: vals }));
                      };
                      return (
                        <div className={styles.gridEditor}>
                          <div className={styles.gridEditorCols}>
                            <div className={styles.gridEditorSection}>
                              <strong>Filas</strong>
                              {(cfg.rows || []).map((row: string, ri: number) => (
                                <div key={ri} className={styles.choiceOptionEdit}>
                                  <input type="text" className={styles.choiceInputEdit} value={row}
                                    onChange={e => { const r = [...cfg.rows]; r[ri] = e.target.value; updateGridField('rows', r); }}
                                    placeholder={`Fila ${ri+1}`} />
                                  <button className={styles.removeOptionBtn} onClick={() => { const r = cfg.rows.filter((_:any, i:number) => i !== ri); updateGridField('rows', r); }}><X size={14}/></button>
                                </div>
                              ))}
                              <button className={styles.addOptionBtn} onClick={() => updateGridField('rows', [...cfg.rows, `Fila ${cfg.rows.length+1}`])}><Plus size={14}/> Añadir fila</button>
                            </div>
                            <div className={styles.gridEditorSection}>
                              <strong>Columnas</strong>
                              {(cfg.columns || []).map((col: string, ci: number) => (
                                <div key={ci} className={styles.choiceOptionEdit}>
                                  <input type="text" className={styles.choiceInputEdit} value={col}
                                    onChange={e => { const c = [...cfg.columns]; c[ci] = e.target.value; updateGridField('columns', c); }}
                                    placeholder={`Col ${ci+1}`} />
                                  <button className={styles.removeOptionBtn} onClick={() => { const c = cfg.columns.filter((_:any, i:number) => i !== ci); updateGridField('columns', c); }}><X size={14}/></button>
                                </div>
                              ))}
                              <button className={styles.addOptionBtn} onClick={() => updateGridField('columns', [...cfg.columns, `Columna ${cfg.columns.length+1}`])}><Plus size={14}/> Añadir columna</button>
                            </div>
                          </div>
                          <div className={styles.gridPreview}>
                            <table className={styles.gridTable}>
                              <thead><tr><th></th>{(cfg.columns||[]).map((c:string, ci:number) => <th key={ci}>{c}</th>)}</tr></thead>
                              <tbody>{(cfg.rows||[]).map((r:string, ri:number) => (<tr key={ri}><td>{r}</td>{(cfg.columns||[]).map((_:any, ci:number) => <td key={ci}><input type={q.type === 'checkbox_grid' ? 'checkbox' : 'radio'} disabled /></td>)}</tr>))}</tbody>
                            </table>
                          </div>
                        </div>
                      );
                    })()}
                  </div>

                  <div className={styles.questionFooter}>
                    <label className={styles.requiredToggle}>
                      <input 
                        type="checkbox" 
                        checked={q.is_required} 
                        onChange={(e) => updateQuestion(index, 'is_required', e.target.checked)} 
                      />
                      Obligatoria
                    </label>
                  </div>
                </div>
              ))}

              {questions.length === 0 && (
                <div className={styles.emptyQuestions}>
                  Empieza agregando tu primera pregunta usando el panel inferior.
                </div>
              )}
            </div>
          </div>
        </div>

        <div className={styles.toolbarArea}>
          <h3>Agregar Pregunta</h3>
          <button className={styles.toolBtn} onClick={() => addQuestion('text')}>
            <Plus size={16} /> Texto Libre
          </button>
          <button className={styles.toolBtn} onClick={() => addQuestion('rating')}>
            <Plus size={16} /> Puntuación ★
          </button>
          <button className={styles.toolBtn} onClick={() => addQuestion('choice')}>
            <Plus size={16} /> Opción Múltiple
          </button>
          <button className={styles.toolBtn} onClick={() => addQuestion('checkbox')}>
            <Plus size={16} /> Casillas
          </button>
          <button className={styles.toolBtn} onClick={() => addQuestion('dropdown')}>
            <Plus size={16} /> Desplegable
          </button>
          <button className={styles.toolBtn} onClick={() => addQuestion('linear_scale')}>
            <Plus size={16} /> Escala Lineal
          </button>
          <button className={styles.toolBtn} onClick={() => addQuestion('grid')}>
            <Plus size={16} /> Cuadrícula
          </button>
          <button className={styles.toolBtn} onClick={() => addQuestion('checkbox_grid')}>
            <Plus size={16} /> Cuadrícula (checks)
          </button>
        </div>
      </div>
    </div>
  );
};

export default SurveyBuilder;
