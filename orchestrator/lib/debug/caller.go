package debug

import (
	"runtime"
	"strings"
)

// GetFullCallerName returns the name of the calling function.
// It skips the specified number of stack frames (default is 1 to skip the immediate caller).
func GetFullCallerName(skip ...int) string {
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
	// Extract package name (everything after the last slash, before the first dot)
	lastSlash := strings.LastIndex(fullName, "/")
	if lastSlash != -1 {
		fullName = fullName[lastSlash+1:]
	}
	return fullName
}
