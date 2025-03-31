package policy

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/rs/zerolog"
)

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
}

type RegoModule struct {
	Package string
	Source  string
}

func NewAgent(ctx context.Context, module RegoModule) (Agent, error) {
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
		logger: zerolog.New(os.Stdout),
	}, nil
}

func (m Agent) Allow(ctx context.Context, context any, r *http.Request) error {
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
