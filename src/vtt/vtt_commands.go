package vtt

import (
	"log"
	"strings"
)

func Commands(vtt *VTTService, cmd string) (bool, string) {
	raw := strings.TrimSpace(cmd)
	rawLower := strings.ToLower(raw)

	if name, ok := extractCommandArg(raw, rawLower, []string{"agregar nombre ", "add name "}); ok {
		if name == "" {
			Notification("Simon Dictate", "Comando: agregar nombre (sin nombre)")
			return true, ""
		}
		if vtt.nameCapitalizer != nil {
			if err := vtt.nameCapitalizer.AddFullName(name); err != nil {
				log.Printf("error adding name: %v", err)
				Notification("Simon Dictate", "Comando fallido: agregar nombre "+name)
				return true, ""
			}
			Notification("Simon Dictate", "Comando OK: agregar nombre "+name)
		}
		return true, ""
	}

	if name, ok := extractCommandArg(raw, rawLower, []string{"quitar nombre ", "remove name ", "delete name "}); ok {
		if name == "" {
			Notification("Simon Dictate", "Comando: quitar nombre (sin nombre)")
			return true, ""
		}
		if vtt.nameCapitalizer != nil {
			if err := vtt.nameCapitalizer.RemoveFullName(name); err != nil {
				log.Printf("error removing name: %v", err)
				Notification("Simon Dictate", "Comando fallido: quitar nombre "+name)
				return true, ""
			}
			Notification("Simon Dictate", "Comando OK: quitar nombre "+name)
		}
		return true, ""
	}

	if rawLower == "recargar nombres" || rawLower == "recarga nombres" || rawLower == "reload names" {
		if vtt.nameCapitalizer != nil {
			if err := vtt.nameCapitalizer.Reload(); err != nil {
				log.Printf("error reloading names: %v", err)
				Notification("Simon Dictate", "Comando fallido: recargar nombres")
				return true, ""
			}
			Notification("Simon Dictate", "Comando OK: recargar nombres")
		}
		return true, ""
	}

	intcmd := strings.ToLower(cmd)
	intcmd = clean(intcmd)
	switch intcmd {
	case "english", "ingles":
		vtt.SetLanguage("en")
		log.Println("Language set to English")
		Notification("Simon Dictate", "Comando OK: English")
		return true, ""
	case "spanish", "espanol":
		vtt.SetLanguage("es")
		log.Println("Language set to Spanish")
		Notification("Simon Dictate", "Comando OK: Spanish")
		return true, ""
	case "auto":
		if vtt.DictationEnabled {
			vtt.DictationEnabled = false
			log.Println("Dictation disabled")
			Notification("Simon Dictate", "Comando OK: Auto (dictation disabled)")
		} else {
			vtt.DictationEnabled = true
			log.Println("Dictation enabled")
			Notification("Simon Dictate", "Comando OK: Auto (dictation enabled)")
		}
		return true, ""
	case "newline", "nuevalinea":
		Notification("Simon Dictate", "Comando OK: newline")
		return true, "\n"
	default:
		return false, ""
	}
}

func extractCommandArg(raw string, rawLower string, prefixes []string) (string, bool) {
	for _, prefix := range prefixes {
		if strings.HasPrefix(rawLower, prefix) {
			return strings.TrimSpace(raw[len(prefix):]), true
		}
	}
	return "", false
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
