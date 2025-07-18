package discovery

// There's a bug in the code generator in which it generates a parameter with map[string]string, which should've been map[string]interface{}
// When regenerating code, you need to alter it to map[string]interface{}
// This function is there to make sure compilation fails if it's map[string]string
var _ = SearchPresentationsParams{
	Query: &map[string]interface{}{},
}
