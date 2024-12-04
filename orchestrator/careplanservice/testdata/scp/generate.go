//go:generate go run .
package main

import (
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
)

// This can be modified to download files from a branch, if you want the files from prod this can be set to an empty string
const branchName = "/branches/update-examples-and-capabilitystatement"

func main() {
	files := []string{
		"https://build.fhir.org/ig/SanteonNL/shared-care-planning" + branchName + "/Task-cps-task-01.json",
		"https://build.fhir.org/ig/SanteonNL/shared-care-planning" + branchName + "/Bundle-hospitalx-bundle-01.json",
		"https://build.fhir.org/ig/SanteonNL/shared-care-planning" + branchName + "/Task-cps-task-02.json",
		"https://build.fhir.org/ig/SanteonNL/shared-care-planning" + branchName + "/QuestionnaireResponse-cps-qr-telemonitoring-enrollment-criteria.json",
		// TODO: Don't think this is needed
		//"https://build.fhir.org/ig/SanteonNL/shared-care-planning" + branchName + "/Subscription-cps-sub-medicalservicecentre.json",
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
