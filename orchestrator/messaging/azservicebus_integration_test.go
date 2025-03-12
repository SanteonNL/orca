//go:build slowtests

package messaging

import (
	"context"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"strings"
	"testing"
	"time"
)

func TestAzureServiceBusBroker(t *testing.T) {
	dockerNetwork, err := setupDockerNetwork(t)
	require.NoError(t, err)
	sqlServerContainer := setupSQLServer(t, dockerNetwork)
	serviceBus := setupAzureServiceBus(t, dockerNetwork, sqlServerContainer)

	broker, err := newAzureServiceBusBroker(AzureServiceBusConfig{
		ConnectionString: "Endpoint=sb://" + serviceBus + ";SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=SAS_KEY_VALUE;UseDevelopmentEmulator=true;",
	}, []string{"orca-patient-enrollment-events", "not-existing-in-servicebus"})
	require.NoError(t, err)
	// When the container signals ready, the Service Bus emulator actually isn't ready yet.
	// See https://github.com/Azure/azure-service-bus-emulator-installer/issues/35
	// It should be 0.5s, but we wait 3s to be on the safe side (to cope with slow CI).
	time.Sleep(3 * time.Second)
	t.Cleanup(func() {
		// Shutdown in case shutdown test doesn't run
		_ = broker.Close(context.Background())
	})

	t.Run("send message", func(t *testing.T) {
		err := broker.SendMessage(context.Background(), "orca-patient-enrollment-events", &Message{
			Body:        []byte(`{"patient_id": "123"}`),
			ContentType: "application/json",
		})
		require.NoError(t, err)
	})
	t.Run("unknown topic in ORCA", func(t *testing.T) {
		err := broker.SendMessage(context.Background(), "unknown-topic", &Message{
			Body:        []byte(`{"patient_id": "123"}`),
			ContentType: "application/json",
		})
		require.EqualError(t, err, "AzureServiceBus: sender not found (topic=unknown-topic)")
	})
	t.Run("unknown topic in Azure ServiceBus", func(t *testing.T) {
		err := broker.SendMessage(context.Background(), "not-existing-in-servicebus", &Message{
			Body:        []byte(`{"patient_id": "123"}`),
			ContentType: "application/json",
		})
		require.ErrorContains(t, err, "amqp:not-found")
	})
	t.Run("shutdown", func(t *testing.T) {
		err := broker.Close(context.Background())
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
