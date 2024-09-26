package nuts

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/nuts-foundation/go-nuts-client/nuts"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
	"net/http"
)

func (d DutchNutsProfile) HttpClient() *http.Client {
	return &http.Client{
		Transport: &oauth2.Transport{
			TokenSource: nuts.OAuth2TokenSource{
				NutsSubject: d.Config.OwnSubject,
				NutsAPIURL:  d.Config.API.URL,
			},
			MetadataLoader: &oauth2.MetadataLoader{},
			AuthzServerLocators: []oauth2.AuthorizationServerLocator{
				oauth2.ProtectedResourceMetadataLocator,
			},
			Scope: careplancontributor.CarePlanServiceOAuth2Scope,
		},
	}
}
