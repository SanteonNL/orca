package taskengine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine/assets"
	"github.com/rs/zerolog/log"
)

type QuestionnaireLoader interface {
	// Load a questionnaire from a URL. It returns nil if the URL can't be handled by the loader (e.g. file does not exist), or an error if something went wrong (e.g. read or unmarshal error).
	Load(url string) (map[string]interface{}, error)
}

var _ QuestionnaireLoader = &EmbeddedQuestionnaireLoader{}

type EmbeddedQuestionnaireLoader struct {
}

func (e EmbeddedQuestionnaireLoader) Load(targetUrl string) (map[string]interface{}, error) {
	parsedUrl, err := url.Parse(targetUrl)
	if err != nil {
		return nil, fmt.Errorf("could not parse target URL: %w", err)
	}
	// Take last path part, which should translate to a file name
	parts := strings.Split(parsedUrl.Path, "/")
	fileName := parts[len(parts)-1]
	if fileName == "" {
		log.Info().Msgf("Cannot load Questionnaire - No path in URL %s", targetUrl)
		// No path, can't handle this URL
		return nil, nil
	}
	fileName = fileName + ".json"
	asJSON, err := assets.FS.ReadFile(fileName)
	if errors.Is(err, fs.ErrNotExist) {
		log.Debug().Msgf("Embedded asset %s not found", fileName)
		return nil, nil
	} else if err != nil {
		// other error
		return nil, fmt.Errorf("could not read embedded assets: %w", err)
	}
	var asMap map[string]interface{}
	if err := json.Unmarshal(asJSON, &asMap); err != nil {
		return nil, fmt.Errorf("could not unmarshal embedded asset %s: %w", fileName, err)
	}
	return asMap, nil
}
