package httpserv

import (
	"net/http"
	"strings"
)

type Route struct {
	Method     string
	Path       string
	Handler    http.HandlerFunc
	Middleware func(http.HandlerFunc) http.HandlerFunc
}

func RegisterRoutes(mux *http.ServeMux, routes ...Route) {
	for _, route := range routes {
		if route.Handler == nil {
			panic("route handler cannot be nil")
		}
		mux.HandleFunc(strings.Join([]string{route.Method, route.Path}, " "), func(writer http.ResponseWriter, request *http.Request) {
			if route.Middleware != nil {
				route.Middleware(route.Handler)(writer, request)
			} else {
				route.Handler(writer, request)
			}
		})
	}
}

func Chain(middlewares ...func(http.HandlerFunc) http.HandlerFunc) func(http.HandlerFunc) http.HandlerFunc {
	return func(final http.HandlerFunc) http.HandlerFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
