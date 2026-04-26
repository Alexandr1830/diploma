import { useState, useEffect } from 'react'
import {
  adminListRuleSets, adminCreateRuleSet, adminGetRuleSet,
  adminUpdateRuleSet, adminDeleteRuleSet,
  adminCreateRule, adminUpdateRule, adminDeleteRule,
} from '../api/client'
import { SelectNative } from '../components/ui/SelectNative'

// Метаданные про типы правил: как каждый тип рендерится в редакторе и какая
// у него форма params. Синхронизируется с internal/compliance/engine.go —
// если там добавился новый тип, обновить и здесь.
const RULE_TYPES = [
  {
    type: 'must_contain_phrase',
    label: 'Должен содержать фразу',
    blank: { phrase: '', case_sensitive: false },
  },
  {
    type: 'must_not_contain_phrase',
    label: 'Не должен содержать фразу',
    blank: { phrase: '', case_sensitive: false },
  },
  {
    type: 'section_order',
    label: 'Порядок разделов',
    blank: { sections: [''], case_sensitive: false },
  },
  {
    type: 'regex_match',
    label: 'Регулярное выражение',
    blank: { pattern: '', expect: 'match', flags: '' },
  },
  {
    type: 'min_word_count',
    label: 'Минимум слов',
    blank: { min: 100 },
  },
]

const TYPE_LABEL = Object.fromEntries(RULE_TYPES.map((t) => [t.type, t.label]))

export default function AdminRuleSetsPage() {
  const [sets, setSets] = useState([])
  const [selectedId, setSelectedId] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = async () => {
    try {
      setError('')
      const list = await adminListRuleSets()
      setSets(list || [])
    } catch (e) { setError(e.message) }
    finally { setLoading(false) }
  }

  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    const name = prompt('Название набора (например, ГОСТ 7.32-2017):')
    if (!name) return
    try {
      const created = await adminCreateRuleSet({ name, description: '', is_active: true })
      await load()
      setSelectedId(created.id)
    } catch (e) { setError(e.message) }
  }

  const handleDelete = async (id) => {
    if (!confirm('Удалить набор и все его правила?')) return
    try {
      await adminDeleteRuleSet(id)
      if (selectedId === id) setSelectedId(null)
      await load()
    } catch (e) { setError(e.message) }
  }

  if (loading) return <div className="loading-screen">Загрузка...</div>

  return (
    <div className="page">
      <h1>Наборы правил</h1>
      <p className="hint">Каждый набор — отдельный норматив (ГОСТ, регламент). Внутри набора — список правил.</p>
      {error && <div className="error-banner">{error}</div>}

      <div className="two-col" style={{ gridTemplateColumns: '300px 1fr', gap: '1rem', marginTop: '1rem' }}>
        <div className="col-left">
          <div className="discussions-header">
            <h3 style={{ margin: 0 }}>Список</h3>
            <button className="btn btn-sm btn-primary" onClick={handleCreate}>+ набор</button>
          </div>
          {sets.length === 0
            ? <div className="empty-state">Наборов пока нет</div>
            : (
              <ul className="ruleset-list">
                {sets.map((s) => (
                  <li key={s.id}
                      className={`ruleset-list-item ${selectedId === s.id ? 'selected' : ''} ${!s.is_active ? 'inactive' : ''}`}
                      onClick={() => setSelectedId(s.id)}>
                    <div className="ruleset-list-name">{s.name}</div>
                    {!s.is_active && <span className="badge status-archived">неактивен</span>}
                    <button className="btn btn-sm btn-danger ruleset-delete-btn"
                            onClick={(e) => { e.stopPropagation(); handleDelete(s.id) }}>
                      Удалить
                    </button>
                  </li>
                ))}
              </ul>
            )}
        </div>
        <div className="col-right">
          {selectedId
            ? <RuleSetEditor key={selectedId} setId={selectedId} onChanged={load} />
            : <div className="empty-state">Выберите набор слева или создайте новый</div>}
        </div>
      </div>
    </div>
  )
}

// RuleSetEditor — редактор одного набора: имя/описание/активность сверху,
// список правил снизу.
function RuleSetEditor({ setId, onChanged }) {
  const [data, setData] = useState(null)
  const [error, setError] = useState('')

  const reload = async () => {
    try {
      setError('')
      const set = await adminGetRuleSet(setId)
      setData(set)
    } catch (e) { setError(e.message) }
  }
  useEffect(() => { reload() }, [setId])

  const saveMeta = async (patch) => {
    try {
      const updated = await adminUpdateRuleSet(setId, {
        name: data.name, description: data.description || '', is_active: data.is_active,
        ...patch,
      })
      setData({ ...data, ...updated })
      onChanged?.()
    } catch (e) { setError(e.message) }
  }

  if (!data) return <div className="loading-screen" style={{ minHeight: 200 }}>...</div>

  return (
    <div className="ruleset-editor">
      <div className="ruleset-editor-header">
        <input className="ruleset-name-input" value={data.name}
               onChange={(e) => setData({ ...data, name: e.target.value })}
               onBlur={(e) => saveMeta({ name: e.target.value })} />
        <label className="ruleset-active-toggle">
          <input type="checkbox" checked={data.is_active}
                 onChange={(e) => { const v = e.target.checked; setData({ ...data, is_active: v }); saveMeta({ is_active: v }) }} />
          <span>Активен</span>
        </label>
      </div>
      <textarea className="ruleset-desc"
                placeholder="Описание (необязательно)"
                value={data.description || ''}
                onChange={(e) => setData({ ...data, description: e.target.value })}
                onBlur={(e) => saveMeta({ description: e.target.value })} />

      {error && <div className="error-banner">{error}</div>}

      <div className="rules-list">
        <h3 style={{ margin: '0 0 0.5rem' }}>Правила ({data.rules?.length || 0})</h3>
        <QuickAddPanel setId={setId} onAdded={reload} />
        {(data.rules || []).map((rule) => (
          <RuleItem key={rule.id} setId={setId} rule={rule} onChanged={reload} />
        ))}
      </div>
    </div>
  )
}

// QuickAddPanel — кнопки добавления правил по смыслу, а не по техническому типу.
// Каждая открывает минимальную inline-форму (1-2 поля) и сразу создаёт правило.
// Это сильно дешевле, чем общий «выбери тип → заполни параметры» — админ
// думает «нужен раздел Введение», а не «mustContainPhrase + параметры».
function QuickAddPanel({ setId, onAdded }) {
  const [mode, setMode] = useState(null) // null | 'section' | 'forbid' | 'min' | 'order' | 'regex'

  const create = async (payload) => {
    try {
      await adminCreateRule(setId, payload)
      setMode(null)
      onAdded?.()
    } catch (e) { alert(e.message) }
  }

  return (
    <div className="quick-add">
      <div className="quick-add-buttons">
        <button className="btn btn-sm btn-secondary" onClick={() => setMode('section')}>+ Раздел</button>
        <button className="btn btn-sm btn-secondary" onClick={() => setMode('order')}>+ Порядок разделов</button>
        <button className="btn btn-sm btn-secondary" onClick={() => setMode('forbid')}>+ Запрет фразы</button>
        <button className="btn btn-sm btn-secondary" onClick={() => setMode('min')}>+ Минимум слов</button>
        <button className="btn btn-sm btn-secondary" onClick={() => setMode('regex')}>+ Regex</button>
      </div>
      {mode && (
        <div className="quick-add-form">
          {mode === 'section' && <QuickFormSection onSubmit={create} onCancel={() => setMode(null)} />}
          {mode === 'forbid'  && <QuickFormForbid  onSubmit={create} onCancel={() => setMode(null)} />}
          {mode === 'min'     && <QuickFormMin     onSubmit={create} onCancel={() => setMode(null)} />}
          {mode === 'order'   && <QuickFormOrder   onSubmit={create} onCancel={() => setMode(null)} />}
          {mode === 'regex'   && <QuickFormRegex   onSubmit={create} onCancel={() => setMode(null)} />}
        </div>
      )}
    </div>
  )
}

// Каждая Quick*-форма знает только свои 1-2 поля и сама собирает payload в
// формате CreateRuleRequest. Имя правила генерируется автоматически — админ
// в любой момент может его переименовать в общем редакторе ниже.

function QuickFormSection({ onSubmit, onCancel }) {
  const [name, setName] = useState('')
  const submit = (e) => {
    e?.preventDefault()
    if (!name.trim()) return
    onSubmit({
      name: `Раздел «${name}»`,
      rule_type: 'must_contain_phrase',
      params: { phrase: name.trim(), case_sensitive: false },
      severity: 'error',
    })
  }
  return (
    <form className="quick-form-row" onSubmit={submit}>
      <input autoFocus type="text" placeholder="Название раздела (например, Введение)"
             value={name} onChange={(e) => setName(e.target.value)} />
      <button type="submit" className="btn btn-sm btn-primary">Добавить</button>
      <button type="button" className="btn btn-sm btn-secondary" onClick={onCancel}>Отмена</button>
    </form>
  )
}

function QuickFormForbid({ onSubmit, onCancel }) {
  const [phrase, setPhrase] = useState('')
  const submit = (e) => {
    e?.preventDefault()
    if (!phrase.trim()) return
    onSubmit({
      name: `Запрет: «${phrase}»`,
      rule_type: 'must_not_contain_phrase',
      params: { phrase: phrase.trim(), case_sensitive: false },
      severity: 'warning',
    })
  }
  return (
    <form className="quick-form-row" onSubmit={submit}>
      <input autoFocus type="text" placeholder="Запрещённая фраза (например, и т.д.)"
             value={phrase} onChange={(e) => setPhrase(e.target.value)} />
      <button type="submit" className="btn btn-sm btn-primary">Добавить</button>
      <button type="button" className="btn btn-sm btn-secondary" onClick={onCancel}>Отмена</button>
    </form>
  )
}

function QuickFormMin({ onSubmit, onCancel }) {
  const [min, setMin] = useState(1000)
  const submit = (e) => {
    e?.preventDefault()
    if (!min || min <= 0) return
    onSubmit({
      name: `Не менее ${min} слов`,
      rule_type: 'min_word_count',
      params: { min: Number(min) },
      severity: 'error',
    })
  }
  return (
    <form className="quick-form-row" onSubmit={submit}>
      <input autoFocus type="number" min={1} placeholder="мин. слов"
             value={min} onChange={(e) => setMin(e.target.value)} />
      <button type="submit" className="btn btn-sm btn-primary">Добавить</button>
      <button type="button" className="btn btn-sm btn-secondary" onClick={onCancel}>Отмена</button>
    </form>
  )
}

function QuickFormOrder({ onSubmit, onCancel }) {
  const [text, setText] = useState('')
  const submit = (e) => {
    e?.preventDefault()
    const sections = text.split(',').map((s) => s.trim()).filter(Boolean)
    if (sections.length < 2) return
    onSubmit({
      name: `Порядок: ${sections.join(' → ')}`,
      rule_type: 'section_order',
      params: { sections, case_sensitive: false },
      severity: 'error',
    })
  }
  return (
    <form className="quick-form-row quick-form-wide" onSubmit={submit}>
      <input autoFocus type="text"
             placeholder="Через запятую: Содержание, Введение, Заключение, Список источников"
             value={text} onChange={(e) => setText(e.target.value)} />
      <button type="submit" className="btn btn-sm btn-primary">Добавить</button>
      <button type="button" className="btn btn-sm btn-secondary" onClick={onCancel}>Отмена</button>
    </form>
  )
}

function QuickFormRegex({ onSubmit, onCancel }) {
  const [pattern, setPattern] = useState('')
  const [expect, setExpect] = useState('match')
  const [flags, setFlags] = useState('')
  const submit = (e) => {
    e?.preventDefault()
    if (!pattern.trim()) return
    onSubmit({
      name: `Regex: ${pattern}`,
      rule_type: 'regex_match',
      params: { pattern, expect, flags },
      severity: 'warning',
    })
  }
  return (
    <form className="quick-form-row quick-form-wide" onSubmit={submit}>
      <input autoFocus type="text" placeholder="шаблон, например \[\d+\]"
             value={pattern} onChange={(e) => setPattern(e.target.value)} />
      <SelectNative value={expect} onChange={(e) => setExpect(e.target.value)} style={{ minWidth: 180 }}>
        <option value="match">должно совпасть</option>
        <option value="nomatch">не должно совпадать</option>
      </SelectNative>
      <input type="text" placeholder="флаги (i,m,s)" style={{ width: 90 }}
             value={flags} onChange={(e) => setFlags(e.target.value)} />
      <button type="submit" className="btn btn-sm btn-primary">Добавить</button>
      <button type="button" className="btn btn-sm btn-secondary" onClick={onCancel}>Отмена</button>
    </form>
  )
}

// RuleItem — одно правило с формой параметров под его тип. Сохраняется при
// потере фокуса (blur), отдельной кнопки «Сохранить» нет.
function RuleItem({ setId, rule, onChanged }) {
  const [draft, setDraft] = useState({
    name: rule.name,
    rule_type: rule.rule_type,
    severity: rule.severity,
    position: rule.position,
    params: rule.params || {},
  })

  const save = async () => {
    try {
      await adminUpdateRule(setId, rule.id, draft)
      onChanged?.()
    } catch (e) { alert(e.message) }
  }

  const remove = async () => {
    if (!confirm('Удалить правило?')) return
    try {
      await adminDeleteRule(setId, rule.id)
      onChanged?.()
    } catch (e) { alert(e.message) }
  }

  return (
    <div className="rule-item">
      <div className="rule-item-header">
        <input className="rule-name-input" value={draft.name}
               onChange={(e) => setDraft({ ...draft, name: e.target.value })}
               onBlur={save} />
        <span className="rule-type-badge">{TYPE_LABEL[draft.rule_type] || draft.rule_type}</span>
        <SelectNative value={draft.severity} style={{ width: 130 }}
          onChange={(e) => { const v = e.target.value; setDraft({ ...draft, severity: v }); }} onBlur={save}>
          <option value="error">error</option>
          <option value="warning">warning</option>
        </SelectNative>
        <button className="btn btn-sm btn-danger" onClick={remove}>×</button>
      </div>
      <RuleParamsForm
        type={draft.rule_type}
        params={draft.params}
        onChange={(params) => setDraft({ ...draft, params })}
        onCommit={save}
      />
    </div>
  )
}

// RuleParamsForm выбирает поля по rule_type — у каждого типа своя форма
// параметров.
function RuleParamsForm({ type, params, onChange, onCommit }) {
  switch (type) {
    case 'must_contain_phrase':
    case 'must_not_contain_phrase':
      return (
        <div className="rule-params">
          <input type="text" placeholder="Фраза" value={params.phrase || ''}
                 onChange={(e) => onChange({ ...params, phrase: e.target.value })} onBlur={onCommit} />
          <label className="rule-checkbox">
            <input type="checkbox" checked={!!params.case_sensitive}
                   onChange={(e) => { onChange({ ...params, case_sensitive: e.target.checked }); }} onBlur={onCommit} />
            учитывать регистр
          </label>
        </div>
      )
    case 'section_order': {
      const sections = Array.isArray(params.sections) ? params.sections : ['']
      const updateSection = (i, value) => {
        const next = sections.slice()
        next[i] = value
        onChange({ ...params, sections: next })
      }
      const addSection = () => onChange({ ...params, sections: [...sections, ''] })
      const removeSection = (i) => onChange({ ...params, sections: sections.filter((_, j) => j !== i) })
      return (
        <div className="rule-params">
          <div className="rule-sections-list">
            {sections.map((s, i) => (
              <div key={i} className="rule-section-row">
                <span className="rule-section-num">{i + 1}.</span>
                <input type="text" placeholder="Название раздела" value={s}
                       onChange={(e) => updateSection(i, e.target.value)} onBlur={onCommit} />
                <button type="button" className="btn btn-sm btn-secondary" onClick={() => { removeSection(i); onCommit() }}>×</button>
              </div>
            ))}
          </div>
          <button type="button" className="btn btn-sm btn-secondary" onClick={addSection}>+ раздел</button>
          <label className="rule-checkbox">
            <input type="checkbox" checked={!!params.case_sensitive}
                   onChange={(e) => { onChange({ ...params, case_sensitive: e.target.checked }); }} onBlur={onCommit} />
            учитывать регистр
          </label>
        </div>
      )
    }
    case 'regex_match':
      return (
        <div className="rule-params">
          <input type="text" placeholder="Шаблон, например ^\d+\.\s" value={params.pattern || ''}
                 onChange={(e) => onChange({ ...params, pattern: e.target.value })} onBlur={onCommit} />
          <SelectNative value={params.expect || 'match'} style={{ width: 200 }}
            onChange={(e) => { onChange({ ...params, expect: e.target.value }); }} onBlur={onCommit}>
            <option value="match">должно совпасть</option>
            <option value="nomatch">не должно совпадать</option>
          </SelectNative>
          <input type="text" placeholder="флаги (i,m,s)" style={{ width: 80 }} value={params.flags || ''}
                 onChange={(e) => onChange({ ...params, flags: e.target.value })} onBlur={onCommit} />
        </div>
      )
    case 'min_word_count':
      return (
        <div className="rule-params">
          <input type="number" min={1} placeholder="мин. слов" value={params.min ?? ''}
                 onChange={(e) => onChange({ ...params, min: Number(e.target.value) || 0 })} onBlur={onCommit} />
        </div>
      )
    default:
      return <div className="hint">неизвестный тип правила</div>
  }
}
