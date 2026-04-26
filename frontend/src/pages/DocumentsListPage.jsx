import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { listDocuments } from '../api/client'

const STATUS_LABELS = {
  draft: 'Черновик', in_review: 'На ревью', needs_revision: 'Доработка',
  approved: 'Одобрен', published: 'Опубликован', archived: 'Архив',
}

export default function DocumentsListPage({ user }) {
  const [docs, setDocs] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    listDocuments()
      .then((data) => setDocs(data || []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="loading-screen">Загрузка документов...</div>
  if (error) return <div className="error-screen">Ошибка: {error}</div>

  return (
    <div className="page">
      <div className="page-header">
        <h1>Документы</h1>
        {(user.role === 'writer' || user.role === 'reviewer' || user.role === 'admin') && (
          <Link to="/documents/create" className="btn btn-primary">Создать документ</Link>
        )}
      </div>

      {docs.length === 0 ? (
        <div className="empty-state">Документов пока нет</div>
      ) : (
        <div className="card-grid">
          {docs.map((doc) => (
            <div key={doc.id} className="doc-card">
              <div className="doc-card-title">{doc.title}</div>
              {doc.description && <div className="doc-card-desc">{doc.description}</div>}
              <div className="doc-card-footer">
                <span className={`badge status-${doc.status}`}>
                  {STATUS_LABELS[doc.status] || doc.status}
                </span>
                <Link to={`/documents/${doc.id}`} className="btn btn-sm btn-primary">Открыть</Link>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
