package subscriptions

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/url"
	"testing"
)

func TestDerivingManager_Notify(t *testing.T) {
	fhirBaseURL, _ := url.Parse("http://example.com/fhir")
	t.Run("CareTeam with multiple (3) subscribers", func(t *testing.T) {
		careTeam := &fhir.CareTeam{
			Id: to.Ptr("10"),
			Participant: []fhir.CareTeamParticipant{
				{Member: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1")},
				{Member: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2")},
				{Member: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "3")},
			},
		}
		ctrl := gomock.NewController(t)
		channelFactory := NewMockChannelFactory(ctrl)

		var capturedMember1Notification coolfhir.SubscriptionNotification
		member1Channel := NewMockChannel(ctrl)
		member1Channel.EXPECT().Notify(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resource interface{}) error {
			capturedMember1Notification = resource.(coolfhir.SubscriptionNotification)
			return nil
		})
		channelFactory.EXPECT().Create(gomock.Any(), *careTeam.Participant[0].Member.Identifier).Return(member1Channel, nil)

		member2Channel := NewMockChannel(ctrl)
		member2Channel.EXPECT().Notify(gomock.Any(), gomock.Any()).Return(nil)
		channelFactory.EXPECT().Create(gomock.Any(), *careTeam.Participant[1].Member.Identifier).Return(member2Channel, nil)

		member3Channel := NewMockChannel(ctrl)
		member3Channel.EXPECT().Notify(gomock.Any(), gomock.Any()).Return(nil)
		channelFactory.EXPECT().Create(gomock.Any(), *careTeam.Participant[2].Member.Identifier).Return(member3Channel, nil)

		manager := DerivingManager{
			FhirBaseURL: fhirBaseURL,
			Channels:    channelFactory,
		}

		err := manager.Notify(context.Background(), careTeam)

		require.NoError(t, err)
		focus, _ := capturedMember1Notification.GetFocus()
		require.Equal(t, "http://example.com/fhir/CareTeam/10", *focus.Reference)
		require.Equal(t, "CareTeam", *focus.Type)
	})
}
