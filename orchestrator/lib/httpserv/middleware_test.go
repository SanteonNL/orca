package httpserv

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterRoutes(t *testing.T) {
	t.Run("should register single route", func(t *testing.T) {
		mux := http.NewServeMux()
		handlerCalled := false

		route := Route{
			Method: "GET",
			Path:   "/test",
			Handler: func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			},
		}

		RegisterRoutes(mux, route)

		// Make request to test route
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("should register multiple routes", func(t *testing.T) {
		mux := http.NewServeMux()
		route1Called := false
		route2Called := false

		routes := []Route{
			{
				Method: "GET",
				Path:   "/test1",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					route1Called = true
					w.WriteHeader(http.StatusOK)
				},
			},
			{
				Method: "POST",
				Path:   "/test2",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					route2Called = true
					w.WriteHeader(http.StatusCreated)
				},
			},
		}

		RegisterRoutes(mux, routes...)

		// Test first route
		req1 := httptest.NewRequest("GET", "/test1", nil)
		w1 := httptest.NewRecorder()
		mux.ServeHTTP(w1, req1)
		assert.True(t, route1Called)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Test second route
		req2 := httptest.NewRequest("POST", "/test2", nil)
		w2 := httptest.NewRecorder()
		mux.ServeHTTP(w2, req2)
		assert.True(t, route2Called)
		assert.Equal(t, http.StatusCreated, w2.Code)
	})

	t.Run("should apply middleware when provided", func(t *testing.T) {
		mux := http.NewServeMux()
		middlewareCalled := false
		handlerCalled := false

		middleware := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				middlewareCalled = true
				next(w, r)
			}
		}

		route := Route{
			Method:     "GET",
			Path:       "/test",
			Handler:    func(w http.ResponseWriter, r *http.Request) { handlerCalled = true },
			Middleware: middleware,
		}

		RegisterRoutes(mux, route)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.True(t, middlewareCalled)
		assert.True(t, handlerCalled)
	})

	t.Run("should panic when handler is nil", func(t *testing.T) {
		mux := http.NewServeMux()

		route := Route{
			Method:  "GET",
			Path:    "/test",
			Handler: nil,
		}

		assert.Panics(t, func() {
			RegisterRoutes(mux, route)
		})
	})

	t.Run("should not apply middleware when nil", func(t *testing.T) {
		mux := http.NewServeMux()
		handlerCalled := false

		route := Route{
			Method:     "GET",
			Path:       "/test",
			Handler:    func(w http.ResponseWriter, r *http.Request) { handlerCalled = true },
			Middleware: nil,
		}

		RegisterRoutes(mux, route)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.True(t, handlerCalled)
	})

	t.Run("should register route with different HTTP methods", func(t *testing.T) {
		mux := http.NewServeMux()

		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
		callOrder := []string{}

		for _, method := range methods {
			currentMethod := method // Capture for closure
			route := Route{
				Method: currentMethod,
				Path:   "/resource",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					callOrder = append(callOrder, currentMethod)
				},
			}
			RegisterRoutes(mux, route)
		}

		// Test each route
		for _, method := range methods {
			req := httptest.NewRequest(method, "/resource", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
		}

		assert.Equal(t, methods, callOrder)
	})
}

func TestChain(t *testing.T) {
	t.Run("should chain single middleware", func(t *testing.T) {
		callOrder := []string{}

		middleware := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, "middleware")
				next(w, r)
			}
		}

		finalHandler := func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "handler")
		}

		chained := Chain(middleware)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		chained(finalHandler)(w, req)

		assert.Equal(t, []string{"middleware", "handler"}, callOrder)
	})

	t.Run("should chain multiple middlewares in reverse order", func(t *testing.T) {
		callOrder := []string{}

		middleware1 := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, "middleware1")
				next(w, r)
			}
		}

		middleware2 := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, "middleware2")
				next(w, r)
			}
		}

		middleware3 := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, "middleware3")
				next(w, r)
			}
		}

		finalHandler := func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "handler")
		}

		chained := Chain(middleware1, middleware2, middleware3)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		chained(finalHandler)(w, req)

		// Middlewares are applied in order (first to last)
		assert.Equal(t, []string{"middleware1", "middleware2", "middleware3", "handler"}, callOrder)
	})

	t.Run("should handle no middlewares", func(t *testing.T) {
		callOrder := []string{}

		finalHandler := func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "handler")
		}

		chained := Chain()
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		chained(finalHandler)(w, req)

		assert.Equal(t, []string{"handler"}, callOrder)
	})

	t.Run("should chain with middleware that modifies request", func(t *testing.T) {
		middlewareModified := false

		middleware := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				middlewareModified = true
				r.Header.Set("X-Custom-Header", "value")
				next(w, r)
			}
		}

		headerValue := ""
		finalHandler := func(w http.ResponseWriter, r *http.Request) {
			headerValue = r.Header.Get("X-Custom-Header")
		}

		chained := Chain(middleware)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		chained(finalHandler)(w, req)

		assert.True(t, middlewareModified)
		assert.Equal(t, "value", headerValue)
	})

	t.Run("should chain with middleware that modifies response", func(t *testing.T) {
		middleware := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Middleware-Header", "middleware-value")
				next(w, r)
			}
		}

		finalHandler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Handler-Header", "handler-value")
			w.WriteHeader(http.StatusOK)
		}

		chained := Chain(middleware)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		chained(finalHandler)(w, req)

		assert.Equal(t, "middleware-value", w.Header().Get("X-Middleware-Header"))
		assert.Equal(t, "handler-value", w.Header().Get("X-Handler-Header"))
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("should chain middlewares that add headers progressively", func(t *testing.T) {
		middleware1 := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Header-1", "value1")
				next(w, r)
			}
		}

		middleware2 := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Header-2", "value2")
				next(w, r)
			}
		}

		finalHandler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Header-3", "value3")
		}

		chained := Chain(middleware1, middleware2)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		chained(finalHandler)(w, req)

		assert.Equal(t, "value1", w.Header().Get("X-Header-1"))
		assert.Equal(t, "value2", w.Header().Get("X-Header-2"))
		assert.Equal(t, "value3", w.Header().Get("X-Header-3"))
	})
}

func TestRegisterRoutesWithChain(t *testing.T) {
	t.Run("should use chained middleware", func(t *testing.T) {
		mux := http.NewServeMux()
		callOrder := []string{}

		middleware1 := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, "middleware1")
				next(w, r)
			}
		}

		middleware2 := func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, "middleware2")
				next(w, r)
			}
		}

		handler := func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "handler")
		}

		route := Route{
			Method:     "GET",
			Path:       "/test",
			Handler:    handler,
			Middleware: Chain(middleware1, middleware2),
		}

		RegisterRoutes(mux, route)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, []string{"middleware1", "middleware2", "handler"}, callOrder)
	})
}
