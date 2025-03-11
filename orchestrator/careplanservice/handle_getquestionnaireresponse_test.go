package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestService_handleGetQuestionnaireResponse(t *testing.T) {
	task1Raw := mustReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw := mustReadFile("./testdata/careplan2-careteam1.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	tests := map[string]struct {
		context       context.Context
		expectedError error
		setup         func(ctx context.Context, client *mock.MockClient)
	}{
		"error: QuestionnaireResponse does not exist": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("fhir error: QuestionnaireResponse not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: QuestionnaireResponse not found"))
			},
		},
		"error: QuestionnaireResponse exists, error fetching task": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("fhir error: no response"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).
					Return(nil)
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: no response"))
			},
		},
		"error: QuestionnaireResponse exists, fetched task, incorrect principal (not task onwer or requester)": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to QuestionnaireResponse",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).
					Return(nil)
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task1Raw},
							},
						}
						return nil
					})
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: no response"))
			},
		},
		"ok: QuestionnaireResponse exists, fetched task, task owner": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).
					Return(nil)
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task1Raw},
							},
						}
						return nil
					})
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			service := &Service{fhirClient: client}
			response, err := service.handleGetQuestionnaireResponse(tt.context, "1", &fhirclient.Headers{})

			if tt.expectedError != nil {
				require.Nil(t, response)
				require.Equal(t, tt.expectedError, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, response)
			}
		})
	}
}
