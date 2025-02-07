package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	err := download("https://raw.githubusercontent.com/Zorgbijjou/scp-homemonitoring/refs/heads/main/fsh-generated/resources/Bundle-zbj-bundle-healthcareservices.json", "testdata/healthcareservice-bundle.json")
	if err != nil {
		panic("Unable to sync healthcareservice-bundle.json: " + err.Error())
	}
	err = download("https://raw.githubusercontent.com/Zorgbijjou/scp-homemonitoring/refs/heads/main/fsh-generated/resources/Bundle-zbj-bundle-questionnaires.json", "testdata/questionnaire-bundle.json")
	if err != nil {
		panic("Unable to sync questionnaire-bundle.json" + err.Error())
	}
	err = download("https://raw.githubusercontent.com/Zorgbijjou/scp-homemonitoring/refs/heads/main/fsh-generated/resources/Questionnaire-zbj-telemonitoring-heartfailure-enrollment.json", "testdata/questionnaire-heartfailure-enrollment.json")
	if err != nil {
		panic("Unable to sync questionnaire-heartfailure-enrollment.json" + err.Error())
	}
}

func download(url string, target string) error {
	println("Downloading", url, "to", target)
	httpResponse, err := http.Get(url)
	if err != nil {
		return err
	}
	if httpResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %s", httpResponse.Status)
	}
	data, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0644)
}
