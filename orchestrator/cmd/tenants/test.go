package tenants

func Test() Config {
	return Config{
		"test": Properties{
			ID:            "test",
			ChipSoftOrgID: "1.2.3.4.5",
			NutsSubject:   "sub",
		},
	}
}
