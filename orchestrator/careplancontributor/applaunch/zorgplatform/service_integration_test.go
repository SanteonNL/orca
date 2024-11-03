package zorgplatform

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
)

func TestService_FetchContext_IntegrationTest(t *testing.T) {
	clientCert, err := tls.LoadX509KeyPair("test-tls-zorgplatform.private.pem", "test-tls-zorgplatform.private.pem")
	if err != nil {
		t.Skip("Skipping TestService_RequestHcpRst_IntegrationTest as test-tls-zorgplatform.private.pem is not present locally")
	}
	signCert, err := tls.LoadX509KeyPair("test-sign-zorgplatform.private.pem", "test-sign-zorgplatform.private.pem")
	if err != nil {
		t.Skip("Skipping TestService_RequestHcpRst_IntegrationTest as test-sign-zorgplatform.private.pem is not present locally")
	}
	service := &Service{
		tlsClientCertificate:  &clientCert,
		signingCertificateKey: signCert.PrivateKey.(*rsa.PrivateKey),
		signingCertificate:    signCert.Certificate,
		zorgplatformHttpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates:  []tls.Certificate{clientCert},
					MinVersion:    tls.VersionTLS12,
					Renegotiation: tls.RenegotiateFreelyAsClient,
				},
			},
		},
		config: Config{
			SAMLRequestTimeout: 10 * time.Second,
			BaseUrl:            "https://zorgplatform.online",
			StsUrl:             "https://zorgplatform.online/sts",
			ApiUrl:             "https://api.zorgplatform.online/fhir/V1",
			SigningConfig: SigningConfig{
				Audience: "https://zorgplatform.online/sts",
				Issuer:   "urn:oid:2.16.840.1.113883.2.4.3.224.1.1",
			},
		},
	}

	workflowId := "b526e773-e1a6-4533-bd00-1360c97e745f"
	launchContext := LaunchContext{
		Practitioner: fhir.Practitioner{Identifier: []fhir.Identifier{
			{
				System: to.Ptr("urn:oid:2.16.840.1.113883.4.1"),
				Value:  to.Ptr("999999999"),
			},
		}},
		WorkflowId: workflowId,
		Bsn:        "999999151", // Assuming Bsn is part of LaunchContext
	}

	accessToken, err := service.RequestHcpRst(launchContext)
	require.NoError(t, err)
	serviceRequest, patient, err := service.fetchContext(context.Background(), accessToken, workflowId)
	require.NoError(t, err)
	require.Nil(t, serviceRequest)
	require.NotNil(t, patient)
}
