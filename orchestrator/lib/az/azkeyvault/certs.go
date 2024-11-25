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

func GetCertificate(ctx context.Context, certClient CertificatesClient, keysClient KeysClient, certificateName string) (*x509.Certificate, *Suite, error) {
	certResponse, err := certClient.GetCertificate(ctx, certificateName, "", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certResponse.CER)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse certificate: %w", err)
	}
	key, err := GetKey(keysClient, certificateName)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get certificate private key: %w", err)
	}
	return cert, key, nil
}

// GetCertificateChain retrieves the full chain from Azure Key Vault
func GetCertificateChain(ctx context.Context, certClient CertificatesClient, keysClient KeysClient, certificateName string) ([][]byte, *Suite, error) {
	certResponse, err := certClient.GetCertificate(ctx, certificateName, "", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get certificate: %w", err)
	}

	// Parse the DER-encoded certificate (which might contain the chain)
	certs, err := x509.ParseCertificates(certResponse.CER)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse certificate chain: %w", err)
	}

	// Initialize a slice to hold the complete certificate chain
	certChain := make([][]byte, 0)

	// Add each parsed certificate to the chain
	for _, cert := range certs {
		certChain = append(certChain, cert.Raw)
	}

	// Retrieve the private key
	key, err := GetKey(keysClient, certificateName)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get certificate private key: %w", err)
	}

	return certChain, key, nil
}

func GetSignatureCertificate(ctx context.Context, certClient CertificatesClient, keysClient KeysClient, certificateName string) (*tls.Certificate, *Suite, error) {
	cert, key, err := GetCertificateChain(ctx, certClient, keysClient, certificateName)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get Signature certificate: %w", err)
	}
	return &tls.Certificate{
		Certificate: cert,
		PrivateKey:  key.SigningKey(),
		// Leaf:        x509.ParseCertificate(cert[0]),
	}, key, nil
}

func GetTLSCertificate(ctx context.Context, certClient CertificatesClient, keysClient KeysClient, certificateName string) (*tls.Certificate, error) {
	cert, key, err := GetCertificate(ctx, certClient, keysClient, certificateName)
	if err != nil {
		return nil, fmt.Errorf("unable to get TLS certificate: %w", err)
	}
	return &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key.SigningKey(),
		Leaf:        cert,
	}, nil
}
