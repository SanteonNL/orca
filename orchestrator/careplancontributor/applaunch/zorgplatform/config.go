package zorgplatform

import (
	"time"
)

func DefaultConfig() Config {
	return Config{
		AzureConfig:        AzureConfig{CredentialType: "managed_identity"},
		SAMLRequestTimeout: 10 * time.Second,
	}
}

type Config struct {
	Enabled            bool          `koanf:"enabled"`
	ApiUrl             string        `koanf:"apiurl"`             //The FHIR API URL
	StsUrl             string        `koanf:"stsurl"`             //The SAML STS URL
	BaseUrl            string        `koanf:"baseurl"`            //The base URL of zorgplatform, can be either their acc or prd URL
	SAMLRequestTimeout time.Duration `koanf:"samlrequesttimeout"` //The timeout for the SAML request, e.g. 10s, 100ms etc
	SigningConfig      SigningConfig `koanf:"sign"`
	DecryptConfig      DecryptConfig `koanf:"decrypt"`
	TaskPerformerUra   string        `koanf:"taskperformerura"`

	AzureConfig    AzureConfig    `koanf:"azure"`
	X509FileConfig X509FileConfig `koanf:"x509"`
}

type TenantConfig struct {
}

type SigningConfig struct {
	Issuer   string `koanf:"iss"`
	Audience string `koanf:"aud"`
}

type DecryptConfig struct {
	Issuer      string `koanf:"iss"`
	Audience    string `koanf:"aud"`
	SignCertPem string `koanf:"signcertpem"`
}

type X509FileConfig struct {
	DecryptCertFile string `koanf:"decryptcertfile"`
	ClientCertFile  string `koanf:"clientcertfile"`
	SignCertFile    string `koanf:"signcertfile"`
	SignKeyFile     string `koanf:"signkeyfile"`
}

type AzureConfig struct {
	KeyVaultConfig AzureKeyVaultConfig `koanf:"keyvault"`
	CredentialType string              `koanf:"credentialtype"`
}

type AzureKeyVaultConfig struct {
	KeyVaultURL     string `koanf:"url"`
	DecryptCertName string `koanf:"decryptcertname"`
	SignCertName    string `koanf:"signcertname"`
	ClientCertName  string `koanf:"clientcertname"`
	AllowInsecure   bool   `koanf:"allowinsecure"`
}
