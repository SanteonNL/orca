package careplancontributor

type Config struct {
}

type AppLaunchConfig struct {
	Demo DemoAppLaunchConfig
}

type DemoAppLaunchConfig struct {
	Enabled bool `koanf:"enabled"`
}
