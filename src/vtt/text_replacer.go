package vtt

import "strings"

// replacementRule maps a sequence of normalized word keys to a fixed output
// string that preserves the desired casing (e.g. brand names).
type replacementRule struct {
	keys        []string
	replacement string
}

// TextReplacer rewrites known word sequences into a canonical form. Matching is
// case-insensitive and accent-insensitive via normalizeKey, mirroring
// KnownTextFilter. It runs after capitalization so the replacement text is kept
// verbatim.
type TextReplacer struct {
	rules []replacementRule
}

func NewTextReplacer() *TextReplacer {
	r := &TextReplacer{}
	// Whisper frequently mishears "AccelOne" as "Axel One" and variants.
	r.AddRule("Axel One", "AccelOne")
	r.AddRule("Axelone", "AccelOne")
	r.AddRule("Excel One", "AccelOne")
	return r
}

func (r *TextReplacer) AddRule(phrase, replacement string) {
	keys := extractWordKeys(phrase)
	if len(keys) == 0 {
		return
	}
	r.rules = append(r.rules, replacementRule{keys: keys, replacement: replacement})
}

func (r *TextReplacer) Apply(text string) string {
	if r == nil || len(r.rules) == 0 {
		return text
	}

	tokens := tokenizeText(text)
	if len(tokens) == 0 {
		return text
	}

	wordKeys := make([]string, 0, len(tokens))
	wordTokenIndexes := make([]int, 0, len(tokens))
	for i, t := range tokens {
		if t.kind == tokenWord {
			wordKeys = append(wordKeys, t.key)
			wordTokenIndexes = append(wordTokenIndexes, i)
		}
	}
	if len(wordKeys) == 0 {
		return text
	}

	// replaceWith[i] != "" means the word token at word-index i emits that text;
	// dropWord[i] means the word token is consumed by a preceding match.
	replaceWith := make([]string, len(wordKeys))
	dropWord := make([]bool, len(wordKeys))

	for _, rule := range r.rules {
		phrase := rule.keys
		if len(phrase) == 0 || len(phrase) > len(wordKeys) {
			continue
		}
		for i := 0; i+len(phrase) <= len(wordKeys); i++ {
			if dropWord[i] || replaceWith[i] != "" {
				continue
			}
			match := true
			for j := 0; j < len(phrase); j++ {
				if wordKeys[i+j] != phrase[j] || dropWord[i+j] {
					match = false
					break
				}
			}
			if match {
				replaceWith[i] = rule.replacement
				for j := 1; j < len(phrase); j++ {
					dropWord[i+j] = true
				}
				i += len(phrase) - 1
			}
		}
	}

	replaceToken := make([]string, len(tokens))
	dropToken := make([]bool, len(tokens))
	for i := range wordKeys {
		if replaceWith[i] != "" {
			replaceToken[wordTokenIndexes[i]] = replaceWith[i]
		}
		if dropWord[i] {
			dropToken[wordTokenIndexes[i]] = true
		}
	}

	var b strings.Builder
	b.Grow(len(text))
	lastWasSpace := true
	for i, t := range tokens {
		if dropToken[i] {
			continue
		}
		if t.kind == tokenSpace {
			if lastWasSpace {
				continue
			}
			b.WriteByte(' ')
			lastWasSpace = true
			continue
		}
		if replaceToken[i] != "" {
			b.WriteString(replaceToken[i])
		} else {
			b.WriteString(t.text)
		}
		lastWasSpace = false
	}

	return strings.TrimSpace(b.String())
}
