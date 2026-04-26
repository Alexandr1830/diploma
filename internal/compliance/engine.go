// Package compliance прогоняет наборы правил по распарсенному тексту версии
// документа. У каждого типа правила свой evaluator, диспетчер выбирает его
// по Rule.Type. Engine не хранит состояния, можно переиспользовать между
// запросами.
package compliance

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"diploma/internal/models"
)

type Engine struct{}

func New() *Engine { return &Engine{} }

// EvaluateAll прогоняет каждое правило по тексту и возвращает RuleResult в
// порядке правил. Правило со сломанным params не валит весь прогон — у него
// просто Passed=false и описание ошибки в Message.
func (e *Engine) EvaluateAll(rules []models.Rule, text string) []models.RuleResult {
	out := make([]models.RuleResult, 0, len(rules))
	for _, rule := range rules {
		out = append(out, e.evaluate(rule, text))
	}
	return out
}

func (e *Engine) evaluate(rule models.Rule, text string) models.RuleResult {
	res := models.RuleResult{
		RuleID:   rule.ID,
		Name:     rule.Name,
		Type:     rule.Type,
		Severity: rule.Severity,
	}
	switch rule.Type {
	case models.RuleMustContainPhrase:
		evalMustContainPhrase(rule, text, &res)
	case models.RuleMustNotContainPhrase:
		evalMustNotContainPhrase(rule, text, &res)
	case models.RuleSectionOrder:
		evalSectionOrder(rule, text, &res)
	case models.RuleRegexMatch:
		evalRegexMatch(rule, text, &res)
	case models.RuleMinWordCount:
		evalMinWordCount(rule, text, &res)
	default:
		res.Passed = false
		res.Message = fmt.Sprintf("неизвестный тип правила: %s", rule.Type)
	}
	return res
}

// Параметры (одна структура на тип правила) и сами evaluator'ы.

type phraseParams struct {
	Phrase        string `json:"phrase"`
	CaseSensitive bool   `json:"case_sensitive"`
}

func evalMustContainPhrase(rule models.Rule, text string, res *models.RuleResult) {
	var p phraseParams
	if err := json.Unmarshal(rule.Params, &p); err != nil || p.Phrase == "" {
		res.Passed = false
		res.Message = `параметр "phrase" обязателен`
		return
	}
	idx := indexOf(text, p.Phrase, p.CaseSensitive)
	if idx >= 0 {
		res.Passed = true
		res.Message = fmt.Sprintf(`фраза "%s" найдена`, p.Phrase)
		res.Location = excerpt(text, idx, len(p.Phrase))
	} else {
		res.Passed = false
		res.Message = fmt.Sprintf(`фраза "%s" не найдена в тексте`, p.Phrase)
	}
}

func evalMustNotContainPhrase(rule models.Rule, text string, res *models.RuleResult) {
	var p phraseParams
	if err := json.Unmarshal(rule.Params, &p); err != nil || p.Phrase == "" {
		res.Passed = false
		res.Message = `параметр "phrase" обязателен`
		return
	}
	idx := indexOf(text, p.Phrase, p.CaseSensitive)
	if idx < 0 {
		res.Passed = true
		res.Message = fmt.Sprintf(`запрещённая фраза "%s" не встречается`, p.Phrase)
	} else {
		res.Passed = false
		res.Message = fmt.Sprintf(`найдена запрещённая фраза "%s"`, p.Phrase)
		res.Location = excerpt(text, idx, len(p.Phrase))
	}
}

type sectionOrderParams struct {
	Sections      []string `json:"sections"`
	CaseSensitive bool     `json:"case_sensitive"`
}

// evalSectionOrder проверяет, что перечисленные разделы встречаются в тексте
// в нужном порядке. Каждый следующий раздел ищется ПОСЛЕ предыдущего.
func evalSectionOrder(rule models.Rule, text string, res *models.RuleResult) {
	var p sectionOrderParams
	if err := json.Unmarshal(rule.Params, &p); err != nil || len(p.Sections) == 0 {
		res.Passed = false
		res.Message = `параметр "sections" должен быть непустым массивом`
		return
	}
	cursor := 0
	for i, section := range p.Sections {
		if section == "" {
			continue
		}
		idx := indexOf(text[cursor:], section, p.CaseSensitive)
		if idx < 0 {
			res.Passed = false
			if i == 0 {
				res.Message = fmt.Sprintf(`раздел "%s" не найден`, section)
			} else {
				res.Message = fmt.Sprintf(`раздел "%s" не найден после "%s" (нарушен порядок)`,
					section, p.Sections[i-1])
			}
			return
		}
		cursor += idx + len(section)
	}
	res.Passed = true
	res.Message = "разделы идут в правильном порядке"
}

type regexMatchParams struct {
	Pattern string `json:"pattern"`
	Expect  string `json:"expect"` // "match" или "nomatch", по умолчанию "match"
	Flags   string `json:"flags"`  // флаги регулярки, например "i" или "is"
}

func evalRegexMatch(rule models.Rule, text string, res *models.RuleResult) {
	var p regexMatchParams
	if err := json.Unmarshal(rule.Params, &p); err != nil || p.Pattern == "" {
		res.Passed = false
		res.Message = `параметр "pattern" обязателен`
		return
	}
	pattern := p.Pattern
	if p.Flags != "" {
		pattern = "(?" + p.Flags + ")" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		res.Passed = false
		res.Message = fmt.Sprintf("регулярка не компилируется: %s", err)
		return
	}
	loc := re.FindStringIndex(text)
	expectMatch := p.Expect != "nomatch"
	matched := loc != nil
	if matched == expectMatch {
		res.Passed = true
		if matched {
			res.Message = "совпадение с шаблоном найдено"
			res.Location = excerpt(text, loc[0], loc[1]-loc[0])
		} else {
			res.Message = "запрещённое совпадение не встречается"
		}
		return
	}
	res.Passed = false
	if expectMatch {
		res.Message = "не найдено ни одного совпадения с шаблоном"
	} else {
		res.Message = "найдено запрещённое совпадение"
		res.Location = excerpt(text, loc[0], loc[1]-loc[0])
	}
}

type minWordCountParams struct {
	Min int `json:"min"`
}

func evalMinWordCount(rule models.Rule, text string, res *models.RuleResult) {
	var p minWordCountParams
	if err := json.Unmarshal(rule.Params, &p); err != nil || p.Min <= 0 {
		res.Passed = false
		res.Message = `параметр "min" должен быть положительным числом`
		return
	}
	count := len(strings.Fields(text))
	if count >= p.Min {
		res.Passed = true
		res.Message = fmt.Sprintf("слов: %d (требуется ≥ %d)", count, p.Min)
	} else {
		res.Passed = false
		res.Message = fmt.Sprintf("слов: %d, требуется минимум %d", count, p.Min)
	}
}

// indexOf возвращает байтовый индекс needle в haystack. caseSensitive=false
// сравнивает по lowercase-варианту.
func indexOf(haystack, needle string, caseSensitive bool) int {
	if caseSensitive {
		return strings.Index(haystack, needle)
	}
	return strings.Index(strings.ToLower(haystack), strings.ToLower(needle))
}

// excerpt возвращает ~80 символов текста вокруг совпадения с многоточиями
// по краям, если кусок обрезан. Идёт в RuleResult.Location, чтобы
// пользователь видел, где именно правило сработало.
func excerpt(text string, byteOffset, byteLen int) string {
	const window = 40
	start := byteOffset - window
	if start < 0 {
		start = 0
	}
	end := byteOffset + byteLen + window
	if end > len(text) {
		end = len(text)
	}
	// Дотягиваем границы до начала рун, чтобы не разрезать UTF-8 посередине.
	for start > 0 && !utf8.RuneStart(text[start]) {
		start--
	}
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end++
	}
	out := strings.TrimSpace(strings.ReplaceAll(text[start:end], "\n", " "))
	if start > 0 {
		out = "…" + out
	}
	if end < len(text) {
		out = out + "…"
	}
	return out
}
