import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { surveyService, Survey } from '../services/surveyService';
import styles from './SurveyViewer.module.css';
import { CheckCircle2 } from 'lucide-react';

const SurveyViewer: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [survey, setSurvey] = useState<Survey | null>(null);
  const [answers, setAnswers] = useState<Record<number, any>>({});
  const [loading, setLoading] = useState(true);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchSurvey = async () => {
      try {
        if (id) {
          const data = await surveyService.getSurvey(parseInt(id));
          setSurvey(data);
        }
      } catch (err) {
        setError('No se pudo cargar la encuesta. Es posible que no exista o no tengas acceso.');
      } finally {
        setLoading(false);
      }
    };
    fetchSurvey();
  }, [id]);

  const handleAnswerChange = (questionId: number, value: any) => {
    setAnswers(prev => ({
      ...prev,
      [questionId]: value
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!survey || !survey.id) return;

    try {
      // Format answers for backend
      const formattedAnswers = Object.entries(answers).map(([qId, value]) => {
        const q = survey.questions?.find(sq => sq.id === parseInt(qId));
        if (q?.type === 'rating') {
          return { question_id: parseInt(qId), number_value: value };
        }
        return { question_id: parseInt(qId), text_value: String(value) };
      });

      await surveyService.submitResponse(survey.id, formattedAnswers);
      setSubmitted(true);
    } catch (err) {
      alert("Hubo un error al enviar tus respuestas. Por favor, intenta de nuevo.");
    }
  };

  if (loading) {
    return <div className={styles.loadingState}>Cargando encuesta...</div>;
  }

  if (error || !survey) {
    return (
      <div className={styles.errorState}>
        <h2>Error</h2>
        <p>{error || 'Encuesta no encontrada'}</p>
        <button onClick={() => navigate('/')}>Volver al inicio</button>
      </div>
    );
  }

  if (submitted) {
    return (
      <div className={styles.successState}>
        <CheckCircle2 size={64} className={styles.successIcon} />
        <h2>¡Gracias por tus respuestas!</h2>
        <p>Tu participación es muy importante para nosotros.</p>
        <button onClick={() => navigate('/dashboard')}>Ir al Panel Principal</button>
      </div>
    );
  }

  return (
    <div className={styles.viewerContainer}>
      <div className={styles.surveyPaper}>
        <header className={styles.surveyHeader}>
          <h1>{survey.title}</h1>
          {survey.description && <p className={styles.surveyDescription}>{survey.description}</p>}
        </header>

        <form onSubmit={handleSubmit} className={styles.surveyForm}>
          {survey.questions?.sort((a, b) => a.order_index - b.order_index).map((q, index) => (
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
                    onChange={(e) => handleAnswerChange(q.id!, e.target.value)}
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
                          onChange={() => handleAnswerChange(q.id!, rating)}
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
                    {(JSON.parse(q.options || '[]') as string[]).map((opt, i) => (
                      <label key={i} className={styles.choiceOption}>
                        <input
                          type="radio"
                          name={`q_${q.id}`}
                          required={q.is_required}
                          checked={answers[q.id!] === opt}
                          onChange={() => handleAnswerChange(q.id!, opt)}
                        />
                        <span className={styles.choiceText}>{opt}</span>
                      </label>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ))}

          <div className={styles.submitArea}>
            <button type="submit" className={styles.submitBtn}>
              Enviar Respuestas
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default SurveyViewer;
