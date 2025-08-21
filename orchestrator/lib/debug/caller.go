package debug

import (
	"runtime"
	"strings"
)

// GetCallerName returns the name of the calling function.
// It skips the specified number of stack frames (default is 1 to skip the immediate caller).
func GetCallerName(skip ...int) string {
	skipFrames := 1
	if len(skip) > 0 {
		skipFrames = skip[0]
	}

	pc, _, _, ok := runtime.Caller(skipFrames)
	if !ok {
		return "unknown"
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}

	fullName := fn.Name()
	// Extract just the function name from the full package path
	parts := strings.Split(fullName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return fullName
}
