package vtt

import "strings"

// KnownTextFilter removes recurring Whisper artifacts based on known phrases.
// Matching is case-insensitive and accent-insensitive via normalizeKey.
type KnownTextFilter struct {
	phrases [][]string
}

func NewKnownTextFilter() *KnownTextFilter {
	f := &KnownTextFilter{}
	// Common recurring artifact reported by users.
	f.AddPhrase("cC por Antarctica Films Argentina")
	return f
}

func (f *KnownTextFilter) AddPhrase(phrase string) {
	keys := extractWordKeys(phrase)
	if len(keys) == 0 {
		return
	}
	f.phrases = append(f.phrases, keys)
}

func (f *KnownTextFilter) Apply(text string) string {
	if f == nil || len(f.phrases) == 0 {
		return strings.TrimSpace(text)
	}

	tokens := tokenizeText(text)
	if len(tokens) == 0 {
		return ""
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
		return strings.TrimSpace(text)
	}

	dropWord := make([]bool, len(wordKeys))
	for _, phrase := range f.phrases {
		if len(phrase) == 0 || len(phrase) > len(wordKeys) {
			continue
		}
		for i := 0; i+len(phrase) <= len(wordKeys); i++ {
			match := true
			for j := 0; j < len(phrase); j++ {
				if wordKeys[i+j] != phrase[j] {
					match = false
					break
				}
			}
			if match {
				for j := 0; j < len(phrase); j++ {
					dropWord[i+j] = true
				}
				i += len(phrase) - 1
			}
		}
	}

	dropToken := make([]bool, len(tokens))
	for i, shouldDrop := range dropWord {
		if shouldDrop {
			dropToken[wordTokenIndexes[i]] = true
		}
	}

	// Keep only one consecutive "gracias" token (e.g. "gracias gracias").
	lastKeptWordKey := ""
	for i, t := range tokens {
		if dropToken[i] {
			continue
		}
		switch t.kind {
		case tokenWord:
			if t.key == "gracias" && lastKeptWordKey == "gracias" {
				dropToken[i] = true
				continue
			}
			lastKeptWordKey = t.key
		case tokenPunct:
			lastKeptWordKey = ""
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

		b.WriteString(t.text)
		lastWasSpace = false
	}

	return strings.TrimSpace(b.String())
}
