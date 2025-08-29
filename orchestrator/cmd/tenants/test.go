package tenants

func Test(mod ...func(*Properties)) Config {
	props := Properties{
		ID: "test",
		ChipSoft: ChipSoftProperties{
			OrganizationID: "1.2.3.4.5",
		},
		Nuts: NutsProperties{
			Subject: "sub",
		},
		EnableImport: true,
	}
	for _, m := range mod {
		m(&props)
	}
	return Config{
		"test": props,
	}
}
