package tenants

import "strconv"

func Sole[T any](tenants map[string]T) (string, T) {
	if len(tenants) != 1 {
		panic("expected exactly one tenant configuration, got: " + strconv.Itoa(len(tenants)))
	}
	for id, cfg := range tenants {
		return id, cfg
	}
	return "", *new(T) // Return zero value if no tenant is found, should not happen with the panic above
}
