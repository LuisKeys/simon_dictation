package vtt

import (
	"log"
	"strings"
)

func Commands(vtt *VTTService, cmd string) (bool, string) {
	raw := strings.TrimSpace(cmd)
	rawLower := strings.ToLower(raw)

	if strings.HasPrefix(rawLower, "agregar nombre ") {
		name := strings.TrimSpace(raw[len("agregar nombre "):])
		if vtt.nameCapitalizer != nil && name != "" {
			if err := vtt.nameCapitalizer.AddFullName(name); err != nil {
				log.Printf("error adding name: %v", err)
				return true, ""
			}
			Notification("Simon Dictate", "Nombre agregado")
		}
		return true, ""
	}

	if strings.HasPrefix(rawLower, "quitar nombre ") {
		name := strings.TrimSpace(raw[len("quitar nombre "):])
		if vtt.nameCapitalizer != nil && name != "" {
			if err := vtt.nameCapitalizer.RemoveFullName(name); err != nil {
				log.Printf("error removing name: %v", err)
				return true, ""
			}
			Notification("Simon Dictate", "Nombre eliminado")
		}
		return true, ""
	}

	if rawLower == "recargar nombres" {
		if vtt.nameCapitalizer != nil {
			if err := vtt.nameCapitalizer.Reload(); err != nil {
				log.Printf("error reloading names: %v", err)
				return true, ""
			}
			Notification("Simon Dictate", "Diccionario de nombres recargado")
		}
		return true, ""
	}

	intcmd := strings.ToLower(cmd)
	intcmd = clean(intcmd)
	switch intcmd {
	case "english", "ingles":
		vtt.SetLanguage("en")
		log.Println("Language set to English")
		Notification("Simon Dictate", "Language set to English")
		return true, ""
	case "spanish", "espanol":
		vtt.SetLanguage("es")
		log.Println("Language set to Spanish")
		Notification("Simon Dictate", "Language set to Spanish")
		return true, ""
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
		return true, ""
	case "newline", "nuevalinea":
		return true, "\n"
	default:
		return false, ""
	}
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
