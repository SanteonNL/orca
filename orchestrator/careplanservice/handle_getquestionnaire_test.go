package careplanservice

import (
	"context"
	"errors"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService_handleGetQuestionnaire(t *testing.T) {
	tests := map[string]struct {
		context       context.Context
		expectedError error
		setup         func(ctx context.Context, client *mock.MockClient)
	}{
		"error: Questionnaire does not exist": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("fhir error: Questionnaire not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Questionnaire/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: Questionnaire not found"))
			},
		},
		"ok: Questionnaire exists, auth": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Questionnaire/1", gomock.Any(), gomock.Any()).
					Return(nil)
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
			questionnaire, err := service.handleGetQuestionnaire(tt.context, "1", &fhirclient.Headers{})

			if tt.expectedError != nil {
				require.Equal(t, tt.expectedError, err)
				require.Nil(t, questionnaire)
			} else {
				require.NoError(t, err)
				require.NotNil(t, questionnaire)
			}
		})
	}
}
