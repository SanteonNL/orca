package must

import "net/url"

func ParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic("invalid URL: " + err.Error())
	}
	return u
}
