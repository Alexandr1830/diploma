import { useState, useId } from 'react'
import { createThread } from '../../api/client'
import { Label } from '../ui/Label'
import { SelectNative } from '../ui/SelectNative'

// createFn — опциональная функция createFn(payload). Если передана, модалка
// использует её вместо стандартного createThread. Нужно для публичных тредов
// в библиотеке: они идут в /library/:id/threads, а не в /versions/:vid/threads.
export default function CreateThreadModal({ docId, versionId, createFn, onClose, onCreated }) {
  const [type, setType] = useState('general')
  const [anchor, setAnchor] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const typeId = useId()

  const handleSubmit = async (e) => {
    e.preventDefault()
    setCreating(true)
    setError('')
    try {
      const payload = { type }
      if (type === 'anchored' && anchor.trim()) {
        payload.anchor = anchor.trim()
      }
      const thread = createFn
        ? await createFn(payload)
        : await createThread(docId, versionId, payload)
      onCreated(thread)
    } catch (err) {
      setError(err.message)
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>Новый тред</h2>
        <form onSubmit={handleSubmit}>
          {error && <div className="error-banner">{error}</div>}

          <div className="form-field">
            <Label htmlFor={typeId}>Тип</Label>
            <SelectNative id={typeId} value={type} onChange={(e) => setType(e.target.value)}>
              <option value="general">Общий</option>
              <option value="anchored">С цитатой</option>
            </SelectNative>
          </div>

          {type === 'anchored' && (
            <label>
              Цитата из документа
              <input
                type="text"
                value={anchor}
                onChange={(e) => setAnchor(e.target.value)}
                placeholder='Например: «раздел 3.1, абзац про требования»'
              />
            </label>
          )}

          <div className="modal-actions">
            <button type="button" className="btn btn-secondary" onClick={onClose}>
              Отмена
            </button>
            <button type="submit" className="btn btn-primary" disabled={creating}>
              {creating ? 'Создание...' : 'Создать'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
