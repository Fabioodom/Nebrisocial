package views

import (
	"fmt"
	"time"
)

// TimeAgo calcula la diferencia de tiempo relativo entre time.Now() y t y devuelve un string en español.
func TimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < 0 {
		return "hace un instante"
	}
	if duration < time.Minute {
		return "hace un instante"
	}
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "hace 1 minuto"
		}
		return fmt.Sprintf("hace %d minutos", minutes)
	}
	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "hace 1 hora"
		}
		return fmt.Sprintf("hace %d horas", hours)
	}
	days := int(duration.Hours() / 24)
	if days == 1 {
		return "hace 1 día"
	}
	return fmt.Sprintf("hace %d días", days)
}

// MsgReplyStyle returns the style attribute value for a chat message, applying extra padding if it is a reply.
func MsgReplyStyle(isReply bool) string {
	if isReply {
		return "animation: fadeInFast 0.25s ease; padding: var(--space-1) 0;"
	}
	return "animation: fadeInFast 0.25s ease;"
}

// MsgAvatarStyle returns the style attribute value for the author avatar link, applying extra font size configuration if it is a reply.
func MsgAvatarStyle(isReply bool) string {
	if isReply {
		return "text-decoration: none; font-size: var(--font-size-sm);"
	}
	return "text-decoration: none; font-size: var(--font-size-md);"
}

// AnonAvatarStyle returns the style attribute value for anonymous avatars, configuring font size based on reply status.
func AnonAvatarStyle(isReply bool) string {
	if isReply {
		return "font-size: var(--font-size-sm);"
	}
	return "font-size: var(--font-size-md);"
}

