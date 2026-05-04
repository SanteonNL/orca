package coolfhir

import (
	"context"
	"errors"
	"net/url"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
)

type stubFHIRClient struct {
	err error
	url *url.URL
}

func (s *stubFHIRClient) Create(resource interface{}, result interface{}, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) CreateWithContext(ctx context.Context, resource interface{}, result interface{}, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) Read(path string, result interface{}, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) ReadWithContext(ctx context.Context, path string, result interface{}, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) Update(path string, resource interface{}, result interface{}, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) UpdateWithContext(ctx context.Context, path string, resource interface{}, result interface{}, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) Delete(path string, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) DeleteWithContext(ctx context.Context, path string, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) Search(resourceType string, params url.Values, result interface{}, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) SearchWithContext(ctx context.Context, resourceType string, params url.Values, result interface{}, opts ...fhirclient.Option) error {
	return s.err
}
func (s *stubFHIRClient) Path(path ...string) *url.URL {
	return s.url
}

func newTestTracedClient(err error) *TracedFHIRClient {
	tp := trace.NewTracerProvider()
	return NewTracedFHIRClient(&stubFHIRClient{err: err}, tp.Tracer("test"))
}

func TestTracedFHIRClient_CreateWithContext(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		err := newTestTracedClient(nil).CreateWithContext(context.Background(), "resource", "result")
		require.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		err := newTestTracedClient(errors.New("create failed")).CreateWithContext(context.Background(), "resource", "result")
		require.Error(t, err)
	})
}

func TestTracedFHIRClient_Create(t *testing.T) {
	err := newTestTracedClient(nil).Create("resource", "result")
	require.NoError(t, err)
}

func TestTracedFHIRClient_ReadWithContext(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		err := newTestTracedClient(nil).ReadWithContext(context.Background(), "Patient/1", "result")
		require.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		err := newTestTracedClient(errors.New("not found")).ReadWithContext(context.Background(), "Patient/1", "result")
		require.Error(t, err)
	})
}

func TestTracedFHIRClient_Read(t *testing.T) {
	err := newTestTracedClient(nil).Read("Patient/1", "result")
	require.NoError(t, err)
}

func TestTracedFHIRClient_UpdateWithContext(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		err := newTestTracedClient(nil).UpdateWithContext(context.Background(), "Patient/1", "resource", "result")
		require.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		err := newTestTracedClient(errors.New("update failed")).UpdateWithContext(context.Background(), "Patient/1", "resource", "result")
		require.Error(t, err)
	})
}

func TestTracedFHIRClient_Update(t *testing.T) {
	err := newTestTracedClient(nil).Update("Patient/1", "resource", "result")
	require.NoError(t, err)
}

func TestTracedFHIRClient_DeleteWithContext(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		err := newTestTracedClient(nil).DeleteWithContext(context.Background(), "Patient/1")
		require.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		err := newTestTracedClient(errors.New("delete failed")).DeleteWithContext(context.Background(), "Patient/1")
		require.Error(t, err)
	})
}

func TestTracedFHIRClient_Delete(t *testing.T) {
	err := newTestTracedClient(nil).Delete("Patient/1")
	require.NoError(t, err)
}

func TestTracedFHIRClient_SearchWithContext(t *testing.T) {
	params := url.Values{"_id": []string{"1"}}
	t.Run("success", func(t *testing.T) {
		err := newTestTracedClient(nil).SearchWithContext(context.Background(), "Patient", params, "result")
		require.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		err := newTestTracedClient(errors.New("search failed")).SearchWithContext(context.Background(), "Patient", params, "result")
		require.Error(t, err)
	})
}

func TestTracedFHIRClient_Search(t *testing.T) {
	params := url.Values{"_id": []string{"1"}}
	err := newTestTracedClient(nil).Search("Patient", params, "result")
	require.NoError(t, err)
}

func TestTracedFHIRClient_Path(t *testing.T) {
	expected, _ := url.Parse("https://example.com/Patient/1")
	tp := trace.NewTracerProvider()
	client := NewTracedFHIRClient(&stubFHIRClient{url: expected}, tp.Tracer("test"))
	result := client.Path("Patient", "1")
	assert.Equal(t, expected, result)
}

func TestTracedFHIRClient_injectTraceContext(t *testing.T) {
	client := newTestTracedClient(nil)
	options := client.injectTraceContext(context.Background(), []fhirclient.Option{})
	assert.NotNil(t, options)
}
