package coolfhir

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
)

// ImportResources imports resources from the given URLs into the FHIR server.
// The URLs must point to JSON documents that contain a valid FHIR Bundle resource.
// The URLs can only contain resources of the given types, otherwise an error is returned.
// If the Bundle was posted to the FHIR server, the HTTP status code is returned.
func ImportResources(ctx context.Context, client fhirclient.Client, resourceTypes []string, urlToLoad string) error {
	var bundleData []byte
	fileUrlPrefix := "file://"
	if strings.HasPrefix(urlToLoad, fileUrlPrefix) {
		filePath := urlToLoad[len(fileUrlPrefix):]
		var err error
		bundleData, err = os.ReadFile(filePath)
		if err != nil {
			return err
		}
	} else {
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, urlToLoad, nil)
		if err != nil {
			return err
		}
		httpResponse, err := http.DefaultClient.Do(httpRequest)
		if err != nil {
			return err
		}
		defer httpResponse.Body.Close()
		if httpResponse.StatusCode <= 199 || httpResponse.StatusCode >= 300 {
			return fmt.Errorf("unexpected status code: %d", httpResponse.StatusCode)
		}
		bundleData, err = io.ReadAll(io.LimitReader(httpResponse.Body, 1024*1024*10+1)) // Limit to 10mb
		if err != nil {
			return err
		}
	}

	// TODO: Might want to make a "secure" http.Client that always does this
	if len(bundleData) > 1024*1024*10 {
		return errors.New("response too large")
	}
	if resourceDesc, err := fhirclient.DescribeResource(bundleData); err != nil {
		return err
	} else if resourceDesc.Type != "Bundle" {
		return fmt.Errorf("not a FHIR Bundle: %s", resourceDesc.Type)
	}
	var bundle fhir.Bundle
	if err := json.Unmarshal(bundleData, &bundle); err != nil {
		return fmt.Errorf("could not unmarshal bundle: %w", err)
	}

	// Check if the Bundle only contains the expected resources
	for _, entry := range bundle.Entry {
		// Sanity check: do not support full URLs, to ease resource type checking
		if strings.Contains(entry.Request.Url, "://") {
			return fmt.Errorf("entry contains a full URL, which is not supported: %s", entry.Request.Url)
		}
		// find characters up until the first non-alphabetical character
		resourceType := entry.Request.Url
		for i, c := range resourceType {
			if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
				resourceType = resourceType[:i]
				break
			}
		}
		if !slices.Contains(resourceTypes, resourceType) {
			return fmt.Errorf("entry contains a resource of an unexpected type: %s", resourceType)
		}
	}

	// Post the Bundle to the FHIR server
	var result fhir.Bundle
	if err := client.CreateWithContext(ctx, bundleData, &result, fhirclient.AtPath("/")); err != nil {
		return fmt.Errorf("import failed: %w", err)
	}
	return nil
}
