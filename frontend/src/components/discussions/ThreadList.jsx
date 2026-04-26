function formatTime(ts) {
  if (!ts) return ''
  return new Date(ts).toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit',
  })
}

// versionByID: опциональная мапа { [version_id]: version_number } — рядом с
// каждым тредом показывается бейдж «v#», чтобы видеть, на какой именно
// версии тред заведён (после загрузки новой версии старые треды никуда не
// деваются).
// unreadIds: Set с id тредов, у которых есть сообщения новее, чем
// пользователь видел в последний раз. На таких тредах подсвечивается жёлтый
// бейдж «новое» — автор сразу видит ответ ревьюера, без ручного обновления.
export default function ThreadList({ threads, selectedId, onSelect, versionByID, unreadIds }) {
  if (!threads || threads.length === 0) return null

  return (
    <div className="thread-list">
      <h3>Обсуждения ({threads.length})</h3>
      {threads.map((t) => {
        const isSelected = t.id === selectedId
        const isResolved = t.status === 'resolved'
        const isUnread = unreadIds && unreadIds.has(t.id) && !isSelected
        const anchor = t.anchor_text?.Valid ? t.anchor_text.String : null
        const verLabel = versionByID && versionByID[t.version_id]

        return (
          <div
            key={t.id}
            className={`thread-item${isSelected ? ' selected' : ''}${isResolved ? ' resolved' : ''}${isUnread ? ' unread' : ''}`}
            onClick={() => onSelect(t.id)}
          >
            <div className="thread-item-top">
              {verLabel && <span className="thread-version-badge">v{verLabel}</span>}
              <span className={`thread-type ${t.thread_type}`}>
                {t.thread_type === 'anchored' ? 'Цитата' : 'Общий'}
              </span>
              <span className={`thread-status ${t.status}`}>{t.status}</span>
              {isUnread && <span className="thread-unread-badge">новое</span>}
            </div>

            {anchor && <div className="thread-anchor">{anchor}</div>}

            {t.last_message_preview ? (
              <div className="thread-preview">{t.last_message_preview}</div>
            ) : (
              <div className="thread-preview empty">Нет сообщений</div>
            )}

            <div className="thread-item-bottom">
              <span>{t.messages_count} сообщ.</span>
              <span>{formatTime(t.last_message_at || t.created_at)}</span>
            </div>
          </div>
        )
      })}
    </div>
  )
}
