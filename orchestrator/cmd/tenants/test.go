package tenants

func Test(mod ...func(*Properties)) Config {
	props := Properties{
		ID:            "test",
		ChipSoftOrgID: "1.2.3.4.5",
		NutsSubject:   "sub",
	}
	for _, m := range mod {
		m(&props)
	}
	return Config{
		"test": props,
	}
}
