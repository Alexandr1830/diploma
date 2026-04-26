import { useEffect, useRef, useState } from 'react'
import { resolveThread } from '../../api/client'

function formatTime(ts) {
  return new Date(ts).toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit',
  })
}

export default function ThreadMessages({ thread, messages, currentUserId, canResolve, onResolved, userNames = {} }) {
  const bottomRef = useRef(null)
  const prevLenRef = useRef(0)
  const [resolving, setResolving] = useState(false)
  const [error, setError] = useState('')

  // Прокручиваем вниз только если сообщений стало БОЛЬШЕ — иначе polling
  // (который заменяет массив той же длины каждые 5 сек) дёргал бы окно вниз
  // на каждом цикле. `block: 'nearest'` ограничивает прокрутку контейнером
  // .messages-list и не двигает страницу целиком.
  useEffect(() => {
    if (messages.length > prevLenRef.current) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }
    prevLenRef.current = messages.length
  }, [messages])

  const handleResolve = async () => {
    setResolving(true)
    setError('')
    try {
      await resolveThread(thread.id)
      onResolved()
    } catch (err) {
      setError(err.message)
    } finally {
      setResolving(false)
    }
  }

  const isResolved = thread.status === 'resolved'
  const anchor = thread.anchor_text?.Valid ? thread.anchor_text.String : null

  return (
    <div className="thread-messages">
      <div className="thread-messages-header">
        <span className={`thread-type ${thread.thread_type}`}>
          {thread.thread_type === 'anchored' ? 'Цитата' : 'Общий'}
        </span>
        <span className={`thread-status ${thread.status}`}>{thread.status === 'open' ? 'Открыт' : 'Закрыт'}</span>
        {canResolve && !isResolved && (
          <button
            className="btn btn-sm btn-danger"
            onClick={handleResolve}
            disabled={resolving}
          >
            {resolving ? '...' : 'Закрыть обсуждение'}
          </button>
        )}
      </div>

      {anchor && <div className="thread-anchor-detail">{anchor}</div>}
      {error && <div className="error-banner">{error}</div>}

      <div className="messages-list">
        {messages.length === 0 ? (
          <div className="empty-state">Сообщений пока нет</div>
        ) : (
          messages.map((m) => {
            const isMine = m.author_id === currentUserId
            return (
              <div key={m.id} className={`message ${isMine ? 'mine' : 'other'}`}>
                <div className="message-meta">
                  <span className="message-author">
                    {isMine ? 'Вы' : (userNames[m.author_id] || `Пользователь #${m.author_id}`)}
                  </span>
                  <span className="message-time">{formatTime(m.created_at)}</span>
                </div>
                <div className="message-text">{m.message_text}</div>
              </div>
            )
          })
        )}
        <div ref={bottomRef} />
      </div>

      {isResolved && (
        <div className="resolved-banner">Тред закрыт для новых сообщений</div>
      )}
    </div>
  )
}
