package utils

// Simple ANSI colors. If you want, we can auto-disable color when not a TTY / NO_COLOR.
func Green(s string) string  { return "\033[32m" + s + "\033[0m" }
func Red(s string) string    { return "\033[31m" + s + "\033[0m" }
func Yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func Cyan(s string) string   { return "\033[36m" + s + "\033[0m" }
