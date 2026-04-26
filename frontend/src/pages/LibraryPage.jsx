import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { listLibrary } from '../api/client'

export default function LibraryPage() {
  const [docs, setDocs] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    listLibrary()
      .then((data) => setDocs(data || []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="loading-screen">Загрузка библиотеки...</div>
  if (error) return <div className="error-screen">Ошибка: {error}</div>

  return (
    <div className="page">
      <h1>Библиотека</h1>
      <p className="subtitle">Опубликованные документы</p>

      {docs.length === 0 ? (
        <div className="empty-state">Опубликованных документов пока нет</div>
      ) : (
        <div className="card-grid">
          {docs.map((doc) => (
            <div key={doc.id} className="doc-card">
              <div className="doc-card-title">{doc.title}</div>
              {doc.description && <div className="doc-card-desc">{doc.description}</div>}
              <div className="doc-card-footer">
                <span className="badge status-published">Опубликован</span>
                <Link to={`/library/${doc.id}`} className="btn btn-sm btn-primary">Открыть</Link>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
