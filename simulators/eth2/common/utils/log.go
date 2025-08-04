package utils

import "fmt"

func Shorten(s string) string {
	start := s[0:6]
	end := s[62:]
	return fmt.Sprintf("%s..%s", start, end)
}

type Logging interface {
	Logf(format string, values ...interface{})
}
