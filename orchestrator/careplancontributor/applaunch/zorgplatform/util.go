package zorgplatform

import "time"

func FormatXSDDateTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func GetCurrentXSDDateTime() string {
	return FormatXSDDateTime(time.Now())
}
