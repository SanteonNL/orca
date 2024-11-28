package taskengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine/assets"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"io/fs"
	"net/url"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type QuestionnaireLoader interface {
	// Load a questionnaire from a URL. It returns nil if the URL can't be handled by the loader (e.g. file does not exist), or an error if something went wrong (e.g. read or unmarshal error).
	Load(ctx context.Context, url string) (*fhir.Questionnaire, error)
}

var _ QuestionnaireLoader = &EmbeddedQuestionnaireLoader{}

type EmbeddedQuestionnaireLoader struct {
}

func (e EmbeddedQuestionnaireLoader) Load(ctx context.Context, targetUrl string) (*fhir.Questionnaire, error) {
	parsedUrl, err := url.Parse(targetUrl)
	if err != nil {
		return nil, fmt.Errorf("could not parse target URL: %w", err)
	}
	// Take last path part, which should translate to a file name
	parts := strings.Split(parsedUrl.Path, "/")
	fileName := parts[len(parts)-1]
	if fileName == "" {
		log.Info().Ctx(ctx).Msgf("Cannot load Questionnaire - No path in URL %s", targetUrl)
		// No path, can't handle this URL
		return nil, nil
	}
	fileName = fileName + ".json"
	asJSON, err := assets.FS.ReadFile(fileName)
	if errors.Is(err, fs.ErrNotExist) {
		log.Debug().Ctx(ctx).Msgf("Embedded asset %s not found", fileName)
		return nil, nil
	} else if err != nil {
		// other error
		return nil, fmt.Errorf("could not read embedded assets: %w", err)
	}
	var result fhir.Questionnaire
	if err := json.Unmarshal(asJSON, &result); err != nil {
		return nil, fmt.Errorf("could not unmarshal embedded asset %s: %w", fileName, err)
	}
	return &result, nil
}

var _ QuestionnaireLoader = FhirApiQuestionnaireLoader{}

type FhirApiQuestionnaireLoader struct {
	client fhirclient.Client
}

func (f FhirApiQuestionnaireLoader) Load(ctx context.Context, u string) (*fhir.Questionnaire, error) {
	isLiteralReference, err := regexp.Match("Questionnaire/[a-zA-Z0-9_-]+", []byte(u))
	if err != nil {
		return nil, err
	}
	var result fhir.Questionnaire
	if isLiteralReference {
		if err := f.client.ReadWithContext(ctx, u, &result); err != nil {
			return nil, err
		}
		return &result, nil
	} else {
		// Assume it's a search operation
		parsedUrl, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		var results fhir.Bundle
		if err := f.client.ReadWithContext(ctx, "Questionnaire", &results, fhirclient.AtUrl(parsedUrl)); err != nil {
			return nil, err
		}
		if len(results.Entry) != 1 {
			return nil, errors.New("expected 1 questionnaire, got " + fmt.Sprint(len(results.Entry)))
		}
		if err := json.Unmarshal(results.Entry[0].Resource, &result); err != nil {
			return nil, fmt.Errorf("could not unmarshal questionnaire (url=%s): %w", u, err)
		}
	}
	return &result, nil
}
