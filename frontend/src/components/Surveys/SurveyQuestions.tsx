import React from 'react';
import styles from './SurveyQuestions.module.css';

/**
 * Formulario de una encuesta: dibuja las preguntas y recoge las respuestas.
 *
 * Está fuera de las páginas porque lo usan dos que no se parecen en nada más:
 * la de dentro de la aplicación (SurveyViewer, con sesión) y la pública que
 * responden las empresas desde el correo (SurveyPublic, sin sesión). Tenerlo
 * duplicado significaría que el día que se agregue un tipo de pregunta, una de
 * las dos se quedaría sin dibujarlo — y sería la pública, que es la que no se
 * abre a diario.
 */

/** Una pregunta tal como llega del servidor, venga del panel o del enlace público. */
export interface RenderableQuestion {
  id?: number;
  text: string;
  type: string;
  options?: string;
  is_required: boolean;
  order_index: number;
}

/** Lo respondido, por id de pregunta. El valor depende del tipo de pregunta. */
export type SurveyAnswers = Record<number, any>;

interface Props {
  questions: RenderableQuestion[];
  answers: SurveyAnswers;
  onChange: (questionId: number, value: any) => void;
  onSubmit: (e: React.FormEvent) => void;
  submitLabel?: string;
  submitting?: boolean;
}

/** Las opciones se guardan como JSON; una encuesta a medio editar no debe romper la página. */
function parseOptions<T>(raw: string | undefined, fallback: T): T {
  try {
    return raw ? (JSON.parse(raw) as T) : fallback;
  } catch {
    return fallback;
  }
}

const SurveyQuestions: React.FC<Props> = ({
  questions,
  answers,
  onChange,
  onSubmit,
  submitLabel = 'Enviar Respuestas',
  submitting = false,
}) => (
  <form onSubmit={onSubmit} className={styles.surveyForm}>
    {[...questions].sort((a, b) => a.order_index - b.order_index).map((q, index) => (
      <div key={q.id || index} className={styles.questionBlock}>
        <label className={styles.questionLabel}>
          <span className={styles.questionNumber}>{index + 1}.</span> {q.text}
          {q.is_required && <span className={styles.requiredMark}>*</span>}
        </label>

        <div className={styles.questionInputArea}>
          {q.type === 'text' && (
            <textarea
              required={q.is_required}
              value={answers[q.id!] || ''}
              onChange={(e) => onChange(q.id!, e.target.value)}
              placeholder="Escribe tu respuesta aquí..."
              className={styles.textInput}
            />
          )}

          {q.type === 'rating' && (
            <div className={styles.ratingGroup}>
              {[1, 2, 3, 4, 5].map(rating => (
                <label key={rating} className={styles.ratingLabel}>
                  <input
                    type="radio"
                    name={`q_${q.id}`}
                    required={q.is_required}
                    checked={answers[q.id!] === rating}
                    onChange={() => onChange(q.id!, rating)}
                  />
                  <span className={styles.ratingStar}>
                    {answers[q.id!] >= rating ? '⭐' : '☆'}
                  </span>
                  <span className={styles.ratingNumber}>{rating}</span>
                </label>
              ))}
            </div>
          )}

          {q.type === 'choice' && (
            <div className={styles.choiceGroup}>
              {parseOptions<string[]>(q.options, []).map((opt, i) => (
                <label key={i} className={styles.choiceOption}>
                  <input
                    type="radio"
                    name={`q_${q.id}`}
                    required={q.is_required}
                    checked={answers[q.id!] === opt}
                    onChange={() => onChange(q.id!, opt)}
                  />
                  <span className={styles.choiceText}>{opt}</span>
                </label>
              ))}
            </div>
          )}

          {q.type === 'checkbox' && (
            <div className={styles.choiceGroup}>
              {parseOptions<string[]>(q.options, []).map((opt, i) => {
                const selected: string[] = Array.isArray(answers[q.id!]) ? answers[q.id!] : [];
                return (
                  <label key={i} className={styles.choiceOption}>
                    <input
                      type="checkbox"
                      checked={selected.includes(opt)}
                      onChange={() => {
                        const next = selected.includes(opt)
                          ? selected.filter(v => v !== opt)
                          : [...selected, opt];
                        onChange(q.id!, next);
                      }}
                    />
                    <span className={styles.choiceText}>{opt}</span>
                  </label>
                );
              })}
            </div>
          )}

          {q.type === 'dropdown' && (
            <select
              required={q.is_required}
              className={styles.textInput}
              style={{ height: 'auto', padding: '10px 14px', cursor: 'pointer' }}
              value={answers[q.id!] || ''}
              onChange={(e) => onChange(q.id!, e.target.value)}
            >
              <option value="">-- Selecciona una opción --</option>
              {parseOptions<string[]>(q.options, []).map((opt, i) => (
                <option key={i} value={opt}>{opt}</option>
              ))}
            </select>
          )}

          {q.type === 'linear_scale' && (() => {
            const cfg = parseOptions<{ min?: number; max?: number; minLabel?: string; maxLabel?: string }>(
              q.options,
              { min: 1, max: 5, minLabel: '', maxLabel: '' },
            );
            const min = cfg.min ?? 1;
            const max = cfg.max ?? 5;
            const nums = Array.from({ length: Math.max(max - min + 1, 0) }, (_, i) => min + i);
            return (
              <div className={styles.linearScaleViewer}>
                <div className={styles.scaleLabels}>
                  {cfg.minLabel && <span>{cfg.minLabel}</span>}
                  <span style={{ flex: 1 }} />
                  {cfg.maxLabel && <span>{cfg.maxLabel}</span>}
                </div>
                <div className={styles.scaleOptions}>
                  {nums.map(n => (
                    <label key={n} className={`${styles.scaleOption} ${answers[q.id!] === n ? styles.scaleOptionActive : ''}`}>
                      <input
                        type="radio"
                        name={`q_${q.id}`}
                        required={q.is_required}
                        checked={answers[q.id!] === n}
                        onChange={() => onChange(q.id!, n)}
                        style={{ display: 'none' }}
                      />
                      {n}
                    </label>
                  ))}
                </div>
              </div>
            );
          })()}

          {(q.type === 'grid' || q.type === 'checkbox_grid') && (() => {
            const cfg = parseOptions<{ rows?: string[]; columns?: string[] }>(q.options, { rows: [], columns: [] });
            const gridAnswers: Record<string, any> = answers[q.id!] || {};
            return (
              <div className={styles.gridViewer}>
                <table className={styles.gridViewerTable}>
                  <thead>
                    <tr>
                      <th></th>
                      {(cfg.columns || []).map((col: string, ci: number) => (
                        <th key={ci}>{col}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {(cfg.rows || []).map((row: string, ri: number) => (
                      <tr key={ri}>
                        <td className={styles.gridRowLabel}>{row}</td>
                        {(cfg.columns || []).map((col: string, ci: number) => (
                          <td key={ci}>
                            <input
                              type={q.type === 'checkbox_grid' ? 'checkbox' : 'radio'}
                              name={q.type === 'grid' ? `q_${q.id}_row_${ri}` : undefined}
                              required={q.is_required && !gridAnswers[row]}
                              checked={q.type === 'checkbox_grid'
                                ? (Array.isArray(gridAnswers[row]) ? gridAnswers[row].includes(col) : false)
                                : gridAnswers[row] === col}
                              onChange={() => {
                                const updated: Record<string, any> = { ...gridAnswers };
                                if (q.type === 'checkbox_grid') {
                                  const prev: string[] = Array.isArray(updated[row]) ? updated[row] : [];
                                  updated[row] = prev.includes(col) ? prev.filter(v => v !== col) : [...prev, col];
                                } else {
                                  updated[row] = col;
                                }
                                onChange(q.id!, updated);
                              }}
                            />
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            );
          })()}
        </div>
      </div>
    ))}

    <div className={styles.submitArea}>
      <button type="submit" className={styles.submitBtn} disabled={submitting}>
        {submitting ? 'Enviando...' : submitLabel}
      </button>
    </div>
  </form>
);

export default SurveyQuestions;
