package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type evalResult struct {
	Allowed  bool
	Resource json.RawMessage
	Subject  *fhir.Reference
}

type searchCache map[*fhir.Reference][]fhir.CarePlan

type Preflight struct {
	ResourceId   string
	Query        url.Values
	ResourceType string
	Principal    fhir.Identifier
	Method       string
	Roles        []string
}

func (preflight Preflight) NewContext(resource json.RawMessage, resourceType string) Context {
	return Context{
		Principal:    preflight.Principal,
		Method:       preflight.Method,
		Roles:        preflight.Roles,
		ResourceType: resourceType,
		Resource:     resource,
	}
}

type Context struct {
	Principal    fhir.Identifier `json:"principal"`
	Method       string          `json:"method"`
	Roles        []string        `json:"roles"`
	ResourceType string          `json:"resource_type"`
	Resource     json.RawMessage `json:"resource"`
	CarePlans    []fhir.CarePlan `json:"careplans"`
}

func subjectToParams(subject *fhir.Reference) (url.Values, error) {
	params := url.Values{}

	if coolfhir.IsLogicalReference(subject) {
		params["patient:Patient.identifier"] = []string{fmt.Sprintf("%s|%s", *subject.Identifier.System, *subject.Identifier.Value)}
	} else if subject.Id != nil {
		params["subject"] = []string{fmt.Sprintf("Patient/%s", *subject.Id)}
	} else if subject.Reference != nil {
		params["subject"] = []string{*subject.Reference}
	} else {
		return params, fmt.Errorf("invalid subject (subject=%+v)", subject)
	}

	return params, nil
}

type Middleware struct {
	agent  Agent
	logger zerolog.Logger
	client fhirclient.Client
}

func NewMiddleware(client fhirclient.Client, agent Agent) Middleware {
	return Middleware{
		agent:  agent,
		logger: agent.logger,
		client: client,
	}
}

func (m Middleware) normalizeCarePlan(ctx context.Context, entry fhir.BundleEntry) (*fhir.CarePlan, error) {
	var carePlan fhir.CarePlan

	if err := json.Unmarshal(entry.Resource, &carePlan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal careplan: %w", err)
	}

	careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
	if err != nil {
		return nil, fmt.Errorf("failed to extract careteam from careplan: %w", err)
	}

	if err := coolfhir.ResolveParticipants(ctx, m.client, careTeam); err != nil {
		return nil, fmt.Errorf("failed to resolve participants: %w", err)
	}

	contained, err := coolfhir.UpdateContainedResource(carePlan.Contained, &carePlan.CareTeam[0], careTeam)
	if err != nil {
		return nil, fmt.Errorf("failed to update contained resource: %w", err)
	}

	carePlan.Contained = contained

	return &carePlan, nil
}

func (m Middleware) preflight(r *http.Request) (*Preflight, error) {
	principal, err := auth.PrincipalFromContext(r.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to extract principal from context: %w", err)
	}

	resourceType := r.PathValue("type")
	id := r.PathValue("id")

	return &Preflight{
		ResourceId:   id,
		ResourceType: resourceType,
		Query:        r.URL.Query(),
		Principal:    principal.Organization.Identifier[0],
		Method:       r.Method,
		Roles:        strings.Split(r.Header.Get("Orca-Auth-Roles"), ","),
	}, nil
}

func (m Middleware) fetchExternalData(ctx context.Context, preflight *Preflight) ([]Context, error) {
	var contexts []Context

	if preflight.ResourceId != "" {
		var resource json.RawMessage

		path := fmt.Sprintf("%s/%s", preflight.ResourceType, preflight.ResourceId)

		if err := m.client.ReadWithContext(ctx, path, &resource); err != nil {
			return nil, fmt.Errorf("failed to read resource: %w", err)
		}

		contexts = append(contexts, preflight.NewContext(resource, preflight.ResourceType))
	} else {
		var bundle fhir.Bundle

		if err := m.client.SearchWithContext(ctx, preflight.ResourceType, preflight.Query, &bundle); err != nil {
			return nil, fmt.Errorf("failed to search for resources: %w", err)
		}

		for _, entry := range bundle.Entry {
			if entry.Resource == nil {
				continue
			}

			var resource coolfhir.RawResource

			if err := json.Unmarshal(entry.Resource, &resource); err != nil {
				return nil, fmt.Errorf("failed to unmarshal resource: %w", err)
			}

			contexts = append(contexts, preflight.NewContext(resource.Raw, resource.Type))
		}
	}

	return contexts, nil
}

func (m Middleware) fetchCarePlans(ctx context.Context, cache searchCache, context *Context, subject *fhir.Reference) error {
	if carePlans, ok := cache[subject]; ok {
		context.CarePlans = carePlans
		return nil
	}

	params, err := subjectToParams(subject)
	if err != nil {
		return fmt.Errorf("failed to convert subject to params: %w", err)
	}

	var bundle fhir.Bundle

	if err := m.client.SearchWithContext(ctx, "CarePlan", params, &bundle); err != nil {
		return fmt.Errorf("failed to search for careplans: %w", err)
	}

	for _, entry := range bundle.Entry {
		if entry.Resource == nil {
			continue
		}

		carePlan, err := m.normalizeCarePlan(ctx, entry)
		if err != nil {
			return fmt.Errorf("failed to resolve careplan: %w", err)
		}

		context.CarePlans = append(context.CarePlans, *carePlan)
	}

	cache[subject] = context.CarePlans

	return nil
}

func (m Middleware) handleRequest(r *http.Request) ([]evalResult, error) {
	preflight, err := m.preflight(r)
	if err != nil {
		return nil, fmt.Errorf("preflight failed: %w", err)
	}

	contexts, err := m.fetchExternalData(r.Context(), preflight)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch external data: %w", err)
	}

	cache := searchCache{}

	var result []evalResult

	for _, context := range contexts {
		subject, err := findSubject(context.Resource, context.ResourceType)
		if err != nil {
			return nil, fmt.Errorf("failed to extract subject: %w", err)
		}

		if subject != nil {
			if err := m.fetchCarePlans(r.Context(), cache, &context, subject); err != nil {
				return nil, fmt.Errorf("failed to fetch careplans: %w", err)
			}
		}

		err = m.agent.Allow(r.Context(), context, r)
		denied := errors.Is(err, ErrAccessDenied)

		if !denied && err != nil {
			return nil, fmt.Errorf("failed to authorize request: %w", err)
		}

		result = append(result, evalResult{
			Allowed:  !denied,
			Subject:  subject,
			Resource: context.Resource,
		})
	}

	return result, nil
}

func (m Middleware) Use(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := m.handleRequest(r)
		if err != nil {
			m.logger.Err(err).Msg("failed to handle request")
			fhirError := coolfhir.NewErrorWithCode("failed to handle request", http.StatusInternalServerError)
			coolfhir.WriteOperationOutcomeFromError(r.Context(), fhirError, "failed to handle request", w)
			return
		}

		if len(result) == 1 && !result[0].Allowed {
			fhirError := coolfhir.NewErrorWithCode("request not allowed", http.StatusForbidden)
			coolfhir.WriteOperationOutcomeFromError(r.Context(), fhirError, "request not allowed due to policy", w)
			return
		}

		next(w, r)
		return
	}
}
