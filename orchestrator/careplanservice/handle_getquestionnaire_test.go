package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestService_handleGetQuestionnaire(t *testing.T) {
	questionnaire := fhir.Questionnaire{
		Id:    to.Ptr("1"),
		Title: to.Ptr("Test Questionnaire"),
	}
	questionnaireRaw, _ := json.Marshal(questionnaire)

	var auditEventRaw []byte
	auditEventRaw, _ = json.Marshal(fhir.AuditEvent{
		Id: to.Ptr("2"),
	})

	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Questionnaire/1"),
					Status:   "200 OK",
				},
				Resource: questionnaireRaw,
			},
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("AuditEvent/2"),
					Status:   "200 OK",
				},
				Resource: auditEventRaw,
			},
		},
	}

	tests := map[string]struct {
		expectedError error
		readError     error
		principal     *auth.Principal
	}{
		"error: Questionnaire does not exist": {
			principal:     auth.TestPrincipal1,
			readError:     errors.New("fhir error: Questionnaire not found"),
			expectedError: errors.New("fhir error: Questionnaire not found"),
		},
		"ok: Questionnaire exists, auth": {
			principal: auth.TestPrincipal1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			client.EXPECT().ReadWithContext(gomock.Any(), "Questionnaire/1", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
				*target.(*fhir.Questionnaire) = questionnaire
				return tt.readError
			}).AnyTimes()

			tx := coolfhir.Transaction()
			service := &Service{fhirClient: client}
			result, err := service.handleGetQuestionnaire(auth.WithPrincipal(context.Background(), *tt.principal), FHIRHandlerRequest{
				ResourceId: "1",
				Principal:  tt.principal,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			}, tx)

			if tt.expectedError != nil {
				require.Len(t, tx.Entry, 0)
				require.Equal(t, tt.expectedError, err)
			} else {
				res, _, err := result(defaultReturnedBundle)
				require.NoError(t, err)
				require.JSONEq(t, string(questionnaireRaw), string(res.Resource))

				require.Len(t, tx.Entry, 2)
				require.Equal(t, "Questionnaire/1", tx.Entry[0].Request.Url)
				require.Equal(t, fhir.HTTPVerbGET, tx.Entry[0].Request.Method)
				require.Equal(t, "AuditEvent", tx.Entry[1].Request.Url)
				require.Equal(t, fhir.HTTPVerbPOST, tx.Entry[1].Request.Method)
			}
		})
	}
}
