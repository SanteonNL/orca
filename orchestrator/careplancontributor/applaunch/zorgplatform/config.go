package zorgplatform

func DefaultConfig() Config {
	return Config{
		AzureConfig: AzureConfig{CredentialType: "managed_identity"},
	}
}

type Config struct {
	Enabled   bool   `koanf:"enabled"`
	PublicKey string `koanf:"publickey"` // ODO
	ApiUrl    string `koanf:"apiurl"`
	StsUrl    string `koanf:"stsurl"`
	Issuer    string `koanf:"iss"`
	OwnIssuer string `koanf:"owniss"` //TODO: Properly distinct between signing and decrypting
	Audience  string `koanf:"aud"`

	AzureConfig    AzureConfig    `koanf:"azure"`
	X509FileConfig X509FileConfig `koanf:"x509"`
}

type X509FileConfig struct {
	DecryptCertFile string `koanf:"decryptcertfile"`
	ClientCertFile  string `koanf:"clientcertfile"`
	SignCertFile    string `koanf:"signcertfile"`
}

type AzureConfig struct {
	KeyVaultConfig AzureKeyVaultConfig `koanf:"keyvault"`
	CredentialType string              `koanf:"credentialtype"`
}

type AzureKeyVaultConfig struct {
	KeyVaultURL        string `koanf:"url"`
	DecryptCertName    string `koanf:"decryptcertname"`
	DecryptCertVersion string `koanf:"decryptcertversion"`
	SignCertName       string `koanf:"signcertname"`
	SignCertVersion    string `koanf:"signcertversion"`
	ClientCertName     string `koanf:"clientcertname"`
	ClientCertVersion  string `koanf:"clientcertversion"`
	AllowInsecure      bool   `koanf:"allowinsecure"`
}
