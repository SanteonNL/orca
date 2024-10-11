package azkeyvault

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
)

func NewCertificatesClient(keyVaultURL string, credentialType string, insecure bool) (*azcertificates.Client, error) {
	cred, err := createCredential(credentialType)
	if err != nil {
		return nil, fmt.Errorf("unable to acquire Azure credential: %w", err)
	}
	var clientOptions *azcertificates.ClientOptions
	if insecure {
		clientOptions = &azcertificates.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				InsecureAllowCredentialWithHTTP: true,
				Transport:                       AzureHttpRequestDoer,
			},
		}
	} else {
		clientOptions = &azcertificates.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: AzureHttpRequestDoer,
			},
		}
	}
	return azcertificates.NewClient(keyVaultURL, cred, clientOptions) // never returns an error
}

func GetTLSCertificate(ctx context.Context, certClient CertificatesClient, keysClient KeysClient, certificateName string, certificateVersion string) (*tls.Certificate, error) {
	certResponse, err := certClient.GetCertificate(ctx, certificateName, certificateVersion, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certResponse.CER)
	if err != nil {
		return nil, fmt.Errorf("unable to parse certificate: %w", err)
	}
	key, err := GetKey(keysClient, certificateName, certificateVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to get certificate private key: %w", err)
	}
	return &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key.SigningKey(),
		Leaf:        cert,
	}, nil
}
