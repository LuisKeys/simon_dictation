package vtt

import (
	"log"
	"strings"
)

func Commands(vtt *VTTService, cmd string) bool {
	intcmd := strings.ToLower(cmd)
	intcmd = clean(intcmd)
	switch intcmd {
	case "english", "ingles":
		vtt.SetLanguage("en")
		log.Println("Language set to English")
		Notification("Simon Dictate", "Language set to English")
	case "spanish", "espanol":
		vtt.SetLanguage("es")
		log.Println("Language set to Spanish")
		Notification("Simon Dictate", "Language set to Spanish")
	case "auto":
		if vtt.DictationEnabled {
			vtt.DictationEnabled = false
			log.Println("Dictation disabled")
			Notification("Simon Dictate", "Dictation disabled")
		} else {
			vtt.DictationEnabled = true
			log.Println("Dictation enabled")
			Notification("Simon Dictate", "Dictation enabled")
		}
	default:
		return false
	}

	return true
}

func clean(cmd string) string {
	// Remove leading/trailing whitespace and convert to lowercase
	cleaned := strings.TrimSpace(strings.ToLower(cmd))
	// Normalize accented characters to plain ASCII equivalents
	cleaned = normalizeAccents(cleaned)
	// Replace multiple spaces with a single space
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	// Remove any character that is not a plain ASCII letter
	var result strings.Builder
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			result.WriteRune(r)
		}
	}
	cleaned = result.String()
	return cleaned
}

func normalizeAccents(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N",
	)
	return replacer.Replace(s)
}
