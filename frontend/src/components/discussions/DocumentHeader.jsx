const STATUS_LABELS = {
  draft: 'Черновик',
  in_review: 'На ревью',
  needs_revision: '��оработка',
  approved: 'Одобрен',
  published: 'Опубликован',
  archived: 'Архив',
}

export default function DocumentHeader({ document: doc, version, user, onRefresh, onCreateThread }) {
  if (!doc) return null

  const reviewerId = doc.reviewer_id?.Valid ? doc.reviewer_id.Int64 : null

  return (
    <div className="doc-header">
      <div className="doc-header-info">
        <h1>{doc.title}</h1>
        {doc.description && <p className="doc-desc">{doc.description}</p>}
        <div className="doc-meta">
          <span className={`badge status-${doc.status}`}>
            {STATUS_LABELS[doc.status] || doc.status}
          </span>
          {version && <span className="meta-item">v{version.version_number} &middot; {version.file_name}</span>}
          <span className="meta-item">Роль: {user.role}</span>
          {reviewerId && <span className="meta-item">Reviewer ID: {reviewerId}</span>}
        </div>
      </div>
      <div className="doc-header-actions">
        <button className="btn btn-secondary" onClick={onRefresh}>Обновить</button>
        {version && (
          <button className="btn btn-primary" onClick={onCreateThread}>Создать тред</button>
        )}
      </div>
    </div>
  )
}
