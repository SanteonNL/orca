package policy

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/rs/zerolog"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

//go:generate mockgen -destination=./policy_agent_mock.go -package=policy -source=agent.go Agent

type PolicyAgent interface {
	Allow(ctx context.Context, context *Context) error
	Preflight(resourceType, id string, r *http.Request) (*Preflight, error)
	PrepareContext(ctx context.Context, cache SearchCache, preflight *Preflight, resource any) (*Context, error)
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

type SearchCache map[*fhir.Reference][]fhir.CarePlan

func NewSearchCache() SearchCache {
	return SearchCache{}
}

//go:embed policy.rego
var source string

var BuiltinModule = RegoModule{
	Package: "policy",
	Source:  source,
}

var ErrAccessDenied = errors.New("request denied by policy")

type Agent struct {
	logger zerolog.Logger
	query  rego.PreparedEvalQuery
	client fhirclient.Client
}

type RegoModule struct {
	Package string
	Source  string
}

type Context struct {
	Principal fhir.Identifier `json:"principal"`
	Method    string          `json:"method"`
	Roles     []string        `json:"roles"`
	Resource  any             `json:"resource"`
	CarePlans []fhir.CarePlan `json:"careplans"`
}

func NewAgent(ctx context.Context, module RegoModule, client fhirclient.Client) (Agent, error) {
	r := rego.New(
		rego.Query(fmt.Sprintf("allow = data.%s.allow", module.Package)),
		rego.Module(module.Package, module.Source),
	)
	query, err := r.PrepareForEval(ctx)
	if err != nil {
		return Agent{}, err
	}

	return Agent{
		query:  query,
		client: client,
		logger: zerolog.New(os.Stdout),
	}, nil
}

func (m Agent) Allow(ctx context.Context, context *Context) error {
	result, err := m.query.Eval(ctx, rego.EvalInput(context))
	if err != nil {
		return fmt.Errorf("failed to evaluate policy: %w", err)
	}

	if len(result) == 0 {
		return fmt.Errorf("policy evaluation failed")
	}

	allow, ok := result[0].Bindings["allow"]
	if !ok {
		return fmt.Errorf("allow binding not found")
	}

	allowed, ok := allow.(bool)
	if !ok {
		return fmt.Errorf("invalid type for allow")
	}

	if !allowed {
		return ErrAccessDenied
	}

	return nil
}

type Preflight struct {
	ResourceId   string
	Query        url.Values
	ResourceType string
	Principal    fhir.Identifier
	Method       string
	Roles        []string
}

func (a Agent) Preflight(resourceType, id string, r *http.Request) (*Preflight, error) {
	principal, err := auth.PrincipalFromContext(r.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to extract principal from context: %w", err)
	}

	var roles []string

	rolesHeader := r.Header.Get("Orca-Auth-Roles")
	if rolesHeader != "" {
		roles = strings.Split(rolesHeader, ",")
	}

	return &Preflight{
		ResourceId:   id,
		ResourceType: resourceType,
		Query:        r.URL.Query(),
		Principal:    principal.Organization.Identifier[0],
		Method:       r.Method,
		Roles:        roles,
	}, nil
}

func (a Agent) PrepareContext(ctx context.Context, cache SearchCache, preflight *Preflight, resource any) (*Context, error) {
	context := Context{
		Principal: preflight.Principal,
		Method:    preflight.Method,
		Roles:     preflight.Roles,
		Resource:  resource,
	}

	subject, err := findSubject(resource, preflight.ResourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to extract subject: %w", err)
	}

	if subject == nil {
		return &context, nil
	}

	if carePlans, ok := cache[subject]; ok {
		context.CarePlans = carePlans
		return &context, nil
	}

	params, err := subjectToParams(subject)
	if err != nil {
		return nil, fmt.Errorf("failed to convert subject to params: %w", err)
	}

	var bundle fhir.Bundle

	if err := a.client.SearchWithContext(ctx, "CarePlan", params, &bundle); err != nil {
		return nil, fmt.Errorf("failed to search for careplans: %w", err)
	}

	for _, entry := range bundle.Entry {
		if entry.Resource == nil {
			continue
		}

		var carePlan fhir.CarePlan

		if err := json.Unmarshal(entry.Resource, &carePlan); err != nil {
			return nil, fmt.Errorf("failed to unmarshal careplan: %w", err)
		}

		context.CarePlans = append(context.CarePlans, carePlan)
	}

	cache[subject] = context.CarePlans

	return &context, nil
}
