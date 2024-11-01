package zorgplatform

import "time"

func FormatXSDDateTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func GetCurrentXSDDateTime() string {
	return FormatXSDDateTime(time.Now())
}
