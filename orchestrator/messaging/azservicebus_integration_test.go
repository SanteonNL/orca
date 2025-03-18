//go:build slowtests

package messaging

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAzureServiceBusBroker(t *testing.T) {
	dockerNetwork, err := setupDockerNetwork(t)
	require.NoError(t, err)
	sqlServerContainer := setupSQLServer(t, dockerNetwork)
	serviceBus := setupAzureServiceBus(t, dockerNetwork, sqlServerContainer)

	queue := Topic{Name: "orca-patient-enrollment-events"}
	broker, err := newAzureServiceBusBroker(AzureServiceBusConfig{
		ConnectionString: "Endpoint=sb://" + serviceBus + ";SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=SAS_KEY_VALUE;UseDevelopmentEmulator=true;",
	}, []Topic{queue, {Name: "not-existing-in-servicebus"}}, "")
	require.NoError(t, err)
	// When the container signals ready, the Service Bus emulator actually isn't ready yet.
	// See https://github.com/Azure/azure-service-bus-emulator-installer/issues/35
	// It should be 0.5s, but we wait 3s to be on the safe side (to cope with slow CI).
	time.Sleep(3 * time.Second)
	ctx := context.Background()
	t.Cleanup(func() {
		// Shutdown in case shutdown test doesn't run
		_ = broker.Close(ctx)
	})

	t.Run("send and receive message", func(t *testing.T) {
		capturedMessages := make(chan Message, 10)
		redeliveryCount := &atomic.Int32{}
		const simulatedDeliveryFailures = 2
		err := broker.Receive(queue, func(_ context.Context, message Message) error {
			if strings.Contains(string(message.Body), "redelivery") {
				if redeliveryCount.Load() < simulatedDeliveryFailures {
					redeliveryCount.Add(1)
					return errors.New("redelivery")
				}
			}
			capturedMessages <- message
			return nil
		})
		require.NoError(t, err)
		t.Run("ok", func(t *testing.T) {
			// Send 2 messages
			err = broker.SendMessage(ctx, queue, &Message{
				Body:        []byte(`{"patient_id": "message 1"}`),
				ContentType: "application/json",
			})
			require.NoError(t, err)
			err = broker.SendMessage(ctx, queue, &Message{
				Body:        []byte(`{"patient_id": "message 2"}`),
				ContentType: "application/json",
			})
			require.NoError(t, err)

			// Expect 2 messages
			message1 := <-capturedMessages
			require.Equal(t, `{"patient_id": "message 1"}`, string(message1.Body))
			message2 := <-capturedMessages
			require.Equal(t, `{"patient_id": "message 2"}`, string(message2.Body))
		})
		t.Run("handler returns not-OK, abandoned for redelivery", func(t *testing.T) {
			err = broker.SendMessage(ctx, queue, &Message{
				Body:        []byte(`redelivery`),
				ContentType: "application/json",
			})
			require.NoError(t, err)

			redeliveredMessage := <-capturedMessages
			require.Equal(t, `redelivery`, string(redeliveredMessage.Body))
		})
	})
	t.Run("unknown topic in ORCA", func(t *testing.T) {
		err := broker.SendMessage(ctx, Topic{Name: "unknown-topic"}, &Message{
			Body:        []byte(`{"patient_id": "123"}`),
			ContentType: "application/json",
		})
		require.EqualError(t, err, "AzureServiceBus: sender not found (topic=unknown-topic)")
	})
	t.Run("unknown topic in Azure ServiceBus", func(t *testing.T) {
		err := broker.SendMessage(ctx, Topic{Name: "not-existing-in-servicebus"}, &Message{
			Body:        []byte(`{"patient_id": "123"}`),
			ContentType: "application/json",
		})
		require.ErrorContains(t, err, "amqp:not-found")
	})
	t.Run("shutdown", func(t *testing.T) {
		err := broker.Close(ctx)
		require.NoError(t, err)
	})
}

func setupSQLServer(t *testing.T, dockerNetwork *tc.DockerNetwork) string {
	t.Log("Starting SQL Server...")
	ctx := context.Background()
	req := tc.ContainerRequest{
		Image:        "mcr.microsoft.com/mssql/server:2022-latest",
		ExposedPorts: []string{"1433/tcp"},
		Networks:     []string{dockerNetwork.Name},
		Env: map[string]string{
			"ACCEPT_EULA":       "Y",
			"MSSQL_SA_PASSWORD": "Z4perS3!cr3!t",
		},
	}
	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			panic(err)
		}
	})
	containerName, err := container.Name(ctx)
	require.NoError(t, err)
	return strings.TrimPrefix(containerName, "/")
}

func setupAzureServiceBus(t *testing.T, dockerNetwork *tc.DockerNetwork, sqlServerHost string) string {
	t.Log("Starting Azure Service Bus...")
	ctx := context.Background()
	const port = "5672/tcp"
	const httpPort = "5300/tcp"
	req := tc.ContainerRequest{
		Image:        "mcr.microsoft.com/azure-messaging/servicebus-emulator:1.1.2",
		ExposedPorts: []string{port, httpPort},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort(port),
			wait.ForListeningPort(httpPort),
			wait.ForHTTP("/health").WithPort(httpPort),
		),
		Networks: []string{dockerNetwork.Name},
		Files: []tc.ContainerFile{
			{
				HostFilePath:      "servicebus-emulator.json",
				ContainerFilePath: "/ServiceBus_Emulator/ConfigFiles/Config.json",
				FileMode:          0444,
			},
		},
		Env: map[string]string{
			"SQL_SERVER":        sqlServerHost,
			"MSSQL_SA_PASSWORD": "Z4perS3!cr3!t",
			"ACCEPT_EULA":       "Y",
			"SQL_WAIT_INTERVAL": "0",
		},
	}
	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			panic(err)
		}
	})
	endpoint, err := container.PortEndpoint(ctx, port, "")
	require.NoError(t, err)
	return endpoint
}

func setupDockerNetwork(t *testing.T) (*tc.DockerNetwork, error) {
	dockerNetwork, err := network.New(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := dockerNetwork.Remove(context.Background()); err != nil {
			panic(err)
		}
	})
	return dockerNetwork, err
}
