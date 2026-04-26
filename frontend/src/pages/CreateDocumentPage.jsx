import { useState, useEffect, useId } from 'react'
import { useNavigate } from 'react-router-dom'
import { createDocument, listUsers } from '../api/client'
import { Label } from '../components/ui/Label'
import { SelectNative } from '../components/ui/SelectNative'

// Admin и reviewer могут задать конкретного writer'а при создании — это
// удобно для случаев, когда документ заводится «за» автора (типичный кейс:
// reviewer открывает документ для writer'а, чтобы тот туда залил материалы).
// Writer создаёт от своего имени и не видит селектор «Автор».
export default function CreateDocumentPage({ user }) {
  const navigate = useNavigate()
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [writerId, setWriterId] = useState('')
  const [reviewerId, setReviewerId] = useState('')
  const [writers, setWriters] = useState([])
  const [reviewers, setReviewers] = useState([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const canPickWriter = user?.role === 'admin' || user?.role === 'reviewer'
  const writerSelectId = useId()
  const reviewerSelectId = useId()

  useEffect(() => {
    // Список ревьюеров нужен всем; список writer'ов — только тем, кто умеет
    // выбирать автора (admin/reviewer).
    listUsers('reviewer').then(setReviewers).catch(() => setReviewers([]))
    if (canPickWriter) {
      listUsers('writer').then(setWriters).catch(() => setWriters([]))
    }
  }, [canPickWriter])

  const handleSubmit = async (e) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const payload = {
        title,
        description,
        project_id: 1,
        category_id: 1,
      }
      if (canPickWriter && writerId) payload.writer_id = Number(writerId)
      if (reviewerId) payload.reviewer_id = Number(reviewerId)
      const doc = await createDocument(payload)
      navigate(`/documents/${doc.id}`)
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page">
      <h1>Создать документ</h1>
      <form className="create-form" onSubmit={handleSubmit}>
        {error && <div className="error-banner">{error}</div>}
        <label>
          Название
          <input type="text" value={title} onChange={(e) => setTitle(e.target.value)} required autoFocus />
        </label>
        <label>
          Описание
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={4} />
        </label>

        {canPickWriter && (
          <div className="form-field">
            <Label htmlFor={writerSelectId}>Автор (writer)</Label>
            <SelectNative id={writerSelectId} value={writerId}
              onChange={(e) => setWriterId(e.target.value)} required>
              <option value="">— выберите автора —</option>
              {writers.map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
            </SelectNative>
            <span className="hint">Документ будет создан от имени выбранного пользователя — он становится владельцем (created_by) и сможет загружать версии.</span>
          </div>
        )}

        <div className="form-field">
          <Label htmlFor={reviewerSelectId}>Ревьюер</Label>
          <SelectNative id={reviewerSelectId} value={reviewerId}
            onChange={(e) => setReviewerId(e.target.value)}>
            <option value="">— не назначен —</option>
            {reviewers.map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
          </SelectNative>
          <span className="hint">Можно назначить позже на странице документа.</span>
        </div>

        <div className="form-actions">
          <button type="button" className="btn btn-secondary" onClick={() => navigate('/documents')}>Отмена</button>
          <button type="submit" className="btn btn-primary" disabled={busy || !title.trim() || (canPickWriter && !writerId)}>{busy ? 'Создание...' : 'Создать'}</button>
        </div>
      </form>
    </div>
  )
}
