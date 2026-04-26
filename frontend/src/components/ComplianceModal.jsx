import { useEffect, useState, useId } from 'react'
import { listActiveRuleSets, runCompliance } from '../api/client'
import { Label } from './ui/Label'
import { SelectNative } from './ui/SelectNative'

// ComplianceModal — модалка проверки документа по нормативу: выбираем
// активный набор правил, жмём «Прогнать», получаем результат по каждому
// правилу. Закрыть без запуска можно. Открывается со страницы документа
// у writer/reviewer/admin; права на сам прогон проверяет backend.
export default function ComplianceModal({ docId, versionId, versionLabel, onClose }) {
  const [sets, setSets] = useState([])
  const [selected, setSelected] = useState('')
  const [check, setCheck] = useState(null)
  const [busy, setBusy] = useState(false)
  const setSelectId = useId()
  const [error, setError] = useState('')

  useEffect(() => {
    const onKey = (e) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  useEffect(() => {
    listActiveRuleSets()
      .then((list) => {
        setSets(list || [])
        if (list && list.length > 0) setSelected(String(list[0].id))
      })
      .catch((e) => setError(e.message))
  }, [])

  const handleRun = async () => {
    if (!selected) return
    setBusy(true); setError(''); setCheck(null)
    try {
      const c = await runCompliance(docId, versionId, Number(selected))
      // results приходит уже массивом — backend сериализует []RuleResult
      // в JSONB через models.JSONB.
      const parsed = typeof c.results === 'string' ? JSON.parse(c.results) : c.results
      setCheck({ ...c, results: parsed })
    } catch (e) { setError(e.message) }
    finally { setBusy(false) }
  }

  return (
    <div className="fullscreen-modal" onClick={onClose}>
      <div className="diff-modal" onClick={(e) => e.stopPropagation()}>
        <div className="fullscreen-header">
          <span className="fullscreen-title">
            Проверка соответствия — {versionLabel}
          </span>
          <button className="fullscreen-close" onClick={onClose} aria-label="Закрыть">×</button>
        </div>
        <div className="diff-modal-body">
          {error && <div className="error-banner">{error}</div>}
          <div className="inline-form">
            <div className="diff-select-label">
              <Label htmlFor={setSelectId}>Норматив</Label>
              <SelectNative id={setSelectId} value={selected}
                onChange={(e) => setSelected(e.target.value)} disabled={busy || sets.length === 0}>
                {sets.length === 0 && <option value="">— нет активных наборов —</option>}
                {sets.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
              </SelectNative>
            </div>
            <button className="btn btn-primary" onClick={handleRun} disabled={busy || !selected}>
              {busy ? 'Проверка...' : 'Запустить'}
            </button>
          </div>

          {check && (
            <div className="compliance-results">
              <div className="compliance-summary">
                <span className={check.failed_rules === 0 ? 'badge status-approved' : 'badge status-needs_revision'}>
                  {check.passed_rules}/{check.total_rules} пройдено
                </span>
                {check.failed_rules > 0 && (
                  <span className="text-muted">{check.failed_rules} нарушений</span>
                )}
              </div>
              <table className="data-table" style={{ marginTop: '0.75rem' }}>
                <thead>
                  <tr>
                    <th style={{ width: 32 }}></th>
                    <th>Правило</th>
                    <th>Сообщение</th>
                  </tr>
                </thead>
                <tbody>
                  {(check.results || []).map((r) => (
                    <tr key={r.rule_id}>
                      <td>{r.passed ? '✓' : (r.severity === 'warning' ? '⚠' : '✗')}</td>
                      <td>
                        <strong>{r.name}</strong>
                        <div className="hint">{r.rule_type}</div>
                      </td>
                      <td>
                        <div>{r.message}</div>
                        {r.location && <div className="compliance-location">«{r.location}»</div>}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
