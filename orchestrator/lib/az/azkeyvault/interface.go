//go:generate mockgen -source=interface.go -destination=./interface_mock.go -package=azkeyvault KeysClient
package azkeyvault

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
)

type KeysClient interface {
	Decrypt(ctx context.Context, keyName string, keyVersion string, parameters azkeys.KeyOperationParameters, options *azkeys.DecryptOptions) (azkeys.DecryptResponse, error)
	GetKey(ctx context.Context, name string, version string, options *azkeys.GetKeyOptions) (azkeys.GetKeyResponse, error)
	Sign(ctx context.Context, name string, version string, parameters azkeys.SignParameters, options *azkeys.SignOptions) (azkeys.SignResponse, error)
}

type CertificatesClient interface {
	GetCertificate(ctx context.Context, certificateName string, certificateVersion string, options *azcertificates.GetCertificateOptions) (azcertificates.GetCertificateResponse, error)
}
