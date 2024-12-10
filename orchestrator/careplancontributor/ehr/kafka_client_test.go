package ehr

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestKafkaClient_SubmitMessage(t *testing.T) {
	tests := []struct {
		name          string
		config        *KafkaConfig
		key           string
		value         string
		expectedError error
	}{
		{
			name: "successful message submission",
			config: &KafkaConfig{
				Enabled: false,
			},
			key:   "test-key",
			value: "test-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client, err := NewClient(*tt.config)
			if err != nil {
				require.EqualError(t, err, tt.expectedError.Error())
				return
			}

			err = client.SubmitMessage(tt.key, tt.value)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
