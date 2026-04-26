import { useEffect, useState } from 'react'
import { getVersionDiff } from '../api/client'

// DiffModal — крупная модалка с diff'ом текста между двумя версиями.
// Удалённые куски подсвечены красным, добавленные — зелёным, общие — серым.
//
// Использование: <DiffModal docId v1 v2 v1Label v2Label onClose />.
export default function DiffModal({ docId, v1, v2, v1Label, v2Label, onClose }) {
  const [diff, setDiff] = useState(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const onKey = (e) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  useEffect(() => {
    let cancelled = false
    setLoading(true); setError(''); setDiff(null)
    getVersionDiff(docId, v1, v2)
      .then((d) => { if (!cancelled) setDiff(d) })
      .catch((e) => { if (!cancelled) setError(e.message) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [docId, v1, v2])

  return (
    <div className="fullscreen-modal" onClick={onClose}>
      <div className="diff-modal" onClick={(e) => e.stopPropagation()}>
        <div className="fullscreen-header">
          <span className="fullscreen-title">
            Сравнение версий: {v1Label} → {v2Label}
          </span>
          <button className="fullscreen-close" onClick={onClose} aria-label="Закрыть">×</button>
        </div>
        <div className="diff-modal-body">
          {loading && <div className="loading-screen" style={{ minHeight: 200 }}>Загрузка...</div>}
          {error && <div className="error-banner">{error}</div>}
          {diff && (
            <>
              {diff.images_changed && (
                <div className="diff-images-notice">Изменения в изображениях обнаружены</div>
              )}
              <div className="diff-legend">
                <span className="diff-legend-item"><span className="diff-removed">удалено</span></span>
                <span className="diff-legend-item"><span className="diff-added">добавлено</span></span>
                <span className="diff-legend-item"><span className="diff-equal">без изменений</span></span>
              </div>
              <pre className="diff-text-pane">
                {diff.text_diff.length === 0
                  ? <span className="text-muted">Распарсенный текст пуст или одинаков</span>
                  : diff.text_diff.map((chunk, i) => (
                      <span key={i} className={`diff-${chunk.type}`}>{chunk.text}</span>
                    ))}
              </pre>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
