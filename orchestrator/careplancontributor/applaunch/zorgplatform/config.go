package zorgplatform

type Config struct {
	Enabled   bool   `koanf:"enabled"`
	PublicKey string `koanf:"publickey"` // ODO
	ApiUrl    string `koanf:"apiurl"`
	StsUrl    string `koanf:"stsurl"`
	Issuer    string `koanf:"iss"`
	Audience  string `koanf:"aud"`

	AzureConfig AzureConfig `koanf:"azure"`
}

type AzureConfig struct {
	KeyVaultConfig AzureKeyVaultConfig `koanf:"keyvault"`
}

type AzureKeyVaultConfig struct {
	KeyVaultURL        string `koanf:"url"`
	DecryptCertName    string `koanf:"decryptcertname"`
	DecryptCertVersion string `koanf:"decryptcertversion"`
	SignCertName       string `koanf:"signcertname"`
	SignCertVersion    string `koanf:"signcertversion"`
	ClientCertName     string `koanf:"clientcertname"`
	ClientCertVersion  string `koanf:"clientcertversion"`
}
