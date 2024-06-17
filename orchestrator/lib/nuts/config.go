package nuts

type Config struct {
	API APIConfig `koanf:"api"`
}

type APIConfig struct {
	Address string `koanf:"address"`
}
