package vtt

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenSpace
	tokenPunct
)

type textToken struct {
	kind tokenKind
	text string
	key  string
}

type NameCapitalizer struct {
	mu               sync.RWMutex
	firstNames       map[string]struct{}
	lastNames        map[string]struct{}
	fullNames        map[string]struct{}
	exceptions       map[string]struct{}
	stopwordsPrev    map[string]struct{}
	namesDir         string
	allowSingleToken bool
}

func NewNameCapitalizer() *NameCapitalizer {
	nc := &NameCapitalizer{
		firstNames: make(map[string]struct{}),
		lastNames:  make(map[string]struct{}),
		fullNames:  make(map[string]struct{}),
		exceptions: make(map[string]struct{}),
		stopwordsPrev: map[string]struct{}{
			"de": {}, "del": {}, "la": {}, "el": {}, "un": {}, "una": {},
			"para": {}, "con": {}, "en": {}, "los": {}, "las": {},
		},
	}
	// Keep precision high by default: only capitalize strong matches
	// (full name and name+surname). Enable single-token names with
	// VTT_CAPITALIZE_SINGLE_NAMES=1 if desired.
	nc.allowSingleToken = os.Getenv("VTT_CAPITALIZE_SINGLE_NAMES") == "1"
	nc.loadDefaults()
	return nc
}

func (nc *NameCapitalizer) loadDefaults() {
	for _, w := range []string{"rosa", "sol", "paz"} {
		nc.exceptions[normalizeKey(w)] = struct{}{}
	}

	dir := os.Getenv("VTT_NAMES_DIR")
	if dir == "" {
		dir = "./vtt_models"
	}
	nc.namesDir = dir

	nc.loadWordFile(filepath.Join(dir, "names_first.txt"), nc.firstNames)
	nc.loadWordFile(filepath.Join(dir, "names_last.txt"), nc.lastNames)
	nc.loadWordFile(filepath.Join(dir, "names_exceptions.txt"), nc.exceptions)
	nc.loadFullNames(filepath.Join(dir, "names_full.txt"))
}

func (nc *NameCapitalizer) Reload() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return nc.reloadLocked()
}

func (nc *NameCapitalizer) reloadLocked() error {

	nc.firstNames = make(map[string]struct{})
	nc.lastNames = make(map[string]struct{})
	nc.fullNames = make(map[string]struct{})
	nc.exceptions = make(map[string]struct{})
	for _, w := range []string{"rosa", "sol", "paz"} {
		nc.exceptions[normalizeKey(w)] = struct{}{}
	}

	nc.loadWordFile(filepath.Join(nc.namesDir, "names_first.txt"), nc.firstNames)
	nc.loadWordFile(filepath.Join(nc.namesDir, "names_last.txt"), nc.lastNames)
	nc.loadWordFile(filepath.Join(nc.namesDir, "names_exceptions.txt"), nc.exceptions)
	nc.loadFullNames(filepath.Join(nc.namesDir, "names_full.txt"))
	return nil
}

func (nc *NameCapitalizer) AddFullName(name string) error {
	norm := normalizeText(name)
	parts := extractWordKeys(norm)
	if len(parts) < 2 {
		return nil
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	key := strings.Join(parts, " ")
	nc.fullNames[key] = struct{}{}
	nc.firstNames[parts[0]] = struct{}{}
	for i := 1; i < len(parts); i++ {
		if nc.isConnector(parts[i]) {
			continue
		}
		nc.lastNames[parts[i]] = struct{}{}
	}

	if err := os.MkdirAll(nc.namesDir, 0o755); err != nil {
		return err
	}
	fpath := filepath.Join(nc.namesDir, "names_full.txt")
	exists := make(map[string]struct{})
	if b, err := os.ReadFile(fpath); err == nil {
		for _, ln := range strings.Split(string(b), "\n") {
			nln := strings.TrimSpace(ln)
			if nln == "" || strings.HasPrefix(nln, "#") {
				continue
			}
			exists[strings.Join(extractWordKeys(normalizeText(nln)), " ")] = struct{}{}
		}
	}
	if _, ok := exists[key]; ok {
		return nil
	}
	fd, err := os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = fd.WriteString(strings.TrimSpace(name) + "\n")
	return err
}

func (nc *NameCapitalizer) RemoveFullName(name string) error {
	norm := normalizeText(name)
	key := strings.Join(extractWordKeys(norm), " ")
	if key == "" {
		return nil
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	delete(nc.fullNames, key)

	fpath := filepath.Join(nc.namesDir, "names_full.txt")
	b, err := os.ReadFile(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(string(b), "\n")
	kept := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			kept = append(kept, ln)
			continue
		}
		if strings.Join(extractWordKeys(normalizeText(t)), " ") == key {
			continue
		}
		kept = append(kept, t)
	}
	if err := os.WriteFile(fpath, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	return nc.reloadLocked()
}

func (nc *NameCapitalizer) loadWordFile(path string, dst map[string]struct{}) {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("warning: unable to read %s: %v", path, err)
		}
		return
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dst[normalizeKey(line)] = struct{}{}
	}
	if err := s.Err(); err != nil {
		log.Printf("warning: scanner error in %s: %v", path, err)
	}
}

func (nc *NameCapitalizer) loadFullNames(path string) {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("warning: unable to read %s: %v", path, err)
		}
		return
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		normLine := normalizeText(line)
		if normLine == "" {
			continue
		}
		parts := extractWordKeys(normLine)
		if len(parts) < 2 {
			continue
		}
		nc.fullNames[strings.Join(parts, " ")] = struct{}{}
		nc.firstNames[parts[0]] = struct{}{}
		for i := 1; i < len(parts); i++ {
			if nc.isConnector(parts[i]) {
				continue
			}
			nc.lastNames[parts[i]] = struct{}{}
		}
	}
	if err := s.Err(); err != nil {
		log.Printf("warning: scanner error in %s: %v", path, err)
	}
}

func (nc *NameCapitalizer) isConnector(s string) bool {
	s = normalizeKey(s)
	return s == "de" || s == "del" || s == "la" || s == "las" || s == "los"
}

func (nc *NameCapitalizer) Apply(text string) string {
	tokens := tokenizeText(text)
	if len(tokens) == 0 {
		return text
	}

	wordTokenIndexes := make([]int, 0, len(tokens))
	for i, t := range tokens {
		if t.kind == tokenWord {
			wordTokenIndexes = append(wordTokenIndexes, i)
		}
	}
	if len(wordTokenIndexes) == 0 {
		return text
	}

	markCap := make([]bool, len(wordTokenIndexes))

	nc.mu.RLock()
	defer nc.mu.RUnlock()

	for i := 0; i < len(wordTokenIndexes); i++ {
		if markCap[i] {
			continue
		}
		for w := 4; w >= 2; w-- {
			if i+w > len(wordTokenIndexes) {
				continue
			}
			parts := make([]string, 0, w)
			for j := i; j < i+w; j++ {
				parts = append(parts, tokens[wordTokenIndexes[j]].key)
			}
			if _, ok := nc.fullNames[strings.Join(parts, " ")]; ok {
				for j := i; j < i+w; j++ {
					markCap[j] = true
				}
				i += (w - 1)
				break
			}
		}
	}

	for i := 0; i < len(wordTokenIndexes)-1; i++ {
		if markCap[i] && markCap[i+1] {
			continue
		}
		w1 := tokens[wordTokenIndexes[i]].key
		w2 := tokens[wordTokenIndexes[i+1]].key

		_, w1First := nc.firstNames[w1]
		_, w2Last := nc.lastNames[w2]
		if w1First && w2Last {
			markCap[i] = true
			markCap[i+1] = true
			continue
		}

		_, w2First := nc.firstNames[w2]
		if w1First && w2First {
			markCap[i] = true
			markCap[i+1] = true
			continue
		}

		_, w1Last := nc.lastNames[w1]
		if w1Last && w2Last && i > 0 && markCap[i-1] {
			markCap[i] = true
			markCap[i+1] = true
		}
	}

	if nc.allowSingleToken {
		for i := 0; i < len(wordTokenIndexes); i++ {
			if markCap[i] {
				continue
			}
			w := tokens[wordTokenIndexes[i]].key
			if _, ok := nc.firstNames[w]; !ok {
				continue
			}
			if _, blocked := nc.exceptions[w]; blocked {
				continue
			}
			if i > 0 {
				prev := tokens[wordTokenIndexes[i-1]].key
				if _, stop := nc.stopwordsPrev[prev]; stop {
					continue
				}
			}
			markCap[i] = true
		}
	}

	for i := 0; i < len(wordTokenIndexes); i++ {
		if !markCap[i] {
			continue
		}
		tokIdx := wordTokenIndexes[i]
		tokens[tokIdx].text = titleWord(tokens[tokIdx].text)
	}

	var b strings.Builder
	b.Grow(len(text) + 8)
	for _, t := range tokens {
		b.WriteString(t.text)
	}
	return b.String()
}

func tokenizeText(text string) []textToken {
	if text == "" {
		return nil
	}
	runes := []rune(text)
	tokens := make([]textToken, 0, len(runes))

	for i := 0; i < len(runes); {
		r := runes[i]
		if unicode.IsSpace(r) {
			j := i + 1
			for j < len(runes) && unicode.IsSpace(runes[j]) {
				j++
			}
			tokens = append(tokens, textToken{kind: tokenSpace, text: string(runes[i:j])})
			i = j
			continue
		}
		if isWordRune(r) {
			j := i + 1
			for j < len(runes) && isWordInnerRune(runes[j]) {
				j++
			}
			w := string(runes[i:j])
			tokens = append(tokens, textToken{kind: tokenWord, text: w, key: normalizeKey(w)})
			i = j
			continue
		}

		tokens = append(tokens, textToken{kind: tokenPunct, text: string(r)})
		i++
	}

	return tokens
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isWordInnerRune(r rune) bool {
	return isWordRune(r) || r == '-' || r == '\''
}

func extractWordKeys(text string) []string {
	toks := tokenizeText(text)
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		if t.kind == tokenWord {
			out = append(out, t.key)
		}
	}
	return out
}

func normalizeKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	r := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u", "Ñ", "n",
	)
	return r.Replace(s)
}

func titleWord(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	upperNext := true
	for i, r := range runes {
		if upperNext && unicode.IsLetter(r) {
			runes[i] = unicode.ToUpper(r)
			upperNext = false
			continue
		}
		if r == '-' || r == '\'' {
			upperNext = true
		} else if unicode.IsLetter(r) {
			runes[i] = unicode.ToLower(r)
		}
	}
	return string(runes)
}
