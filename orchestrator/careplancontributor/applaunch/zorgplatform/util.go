package zorgplatform

import (
	"fmt"
	"time"
)

func FormatXSDDateTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func GetCurrentXSDDateTime() string {
	return FormatXSDDateTime(time.Now())
}

// xmlTimeBetween parsed the given timestamps as xsd:dateTime and returns whether the check time is between the notBefore and notAfter times.
// The notBefore and notAfter times are inclusive.
func xmlTimeBetweenInclusive(notBefore string, notAfter string, check time.Time) (bool, error) {
	nb, err := time.Parse(time.RFC3339Nano, notBefore)
	if err != nil {
		return false, fmt.Errorf("invalid xsd:dateTime: %s", notBefore)
	}
	na, err := time.Parse(time.RFC3339Nano, notAfter)
	if err != nil {
		return false, fmt.Errorf("invalid xsd:dateTime: %s", notAfter)
	}
	return check.Equal(nb) || check.Equal(na) || (check.After(nb) && check.Before(na)), nil
}
