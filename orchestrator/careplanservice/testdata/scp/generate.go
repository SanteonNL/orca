//go:generate go run .
package main

import (
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
)

func main() {
	files := []string{
		"https://santeonnl.github.io/shared-care-planning/Bundle-cps-bundle-01.json",
		"https://santeonnl.github.io/shared-care-planning/Bundle-cps-bundle-02.json",
	}
	for _, fileURL := range files {
		httpResponse, err := http.Get(fileURL)
		if err != nil {
			panic(err)
		}
		if httpResponse.StatusCode != 200 {
			panic("unexpected status code: " + strconv.Itoa(httpResponse.StatusCode))
		}
		defer httpResponse.Body.Close()
		data, _ := io.ReadAll(httpResponse.Body)
		// Write the file to the filesystem, use the last part of the URL as the filename
		// e.g. https://santeonnl.github.io/shared-care-planning/Bundle-cps-bundle-01.json -> Bundle-cps-bundle-01.json
		targetFileName := path.Base(fileURL)
		if targetFileName == "/" || targetFileName == "." {
			panic("invalid target file name")
		}
		if err = os.WriteFile(targetFileName, data, 0644); err != nil {
			panic(err)
		}
	}
}
