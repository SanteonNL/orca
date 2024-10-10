package azkeyvault

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
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

func GetCertificate(ctx context.Context, client *azcertificates.Client, certificateName string, certificateVersion string) (*x509.Certificate, error) {
	certResponse, err := client.GetCertificate(ctx, certificateName, certificateVersion, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get certificate: %w", err)
	}
	result, err := x509.ParseCertificate(certResponse.CER)
	if err != nil {
		return nil, fmt.Errorf("unable to parse certificate: %w", err)
	}
	return result, nil
}

func GetTLSCertificate(ctx context.Context, certsClient *azcertificates.Client, keysClient *azkeys.Client, certificateName string, certificateVersion string) (*tls.Certificate, error) {
	privateKey, err := GetKey(keysClient, certificateName, certificateVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to get private key for TLS certificate: %w", err)
	}
	certificate, err := GetCertificate(ctx, certsClient, certificateName, certificateVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to get certificate for TLS certificate: %w", err)
	}
	return &tls.Certificate{
		Certificate: [][]byte{certificate.Raw},
		Leaf:        certificate,
		PrivateKey:  privateKey,
	}, nil
}
