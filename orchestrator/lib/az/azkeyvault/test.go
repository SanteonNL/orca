package azkeyvault

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
)

func NewTestServer() *TestAzureKeyVault {
	result := &TestAzureKeyVault{
		keys:         make(map[string]crypto.PrivateKey),
		certificates: make(map[string]*tls.Certificate),
	}
	httpServerMux := http.NewServeMux()
	httpServer := httptest.NewTLSServer(httpServerMux)
	httpServerMux.Handle("GET /keys/{name}/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if key, ok := result.keys[name]; ok {
			bundle := result.keyToBundle(name, key)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(bundle)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	httpServerMux.Handle("GET /certificates/{name}/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if cert, ok := result.certificates[name]; ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(azcertificates.Certificate{
				CER: cert.Certificate[0],
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	httpServerMux.Handle("/{rest...}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "unhandled request", slog.String(logging.FieldPath, r.URL.Path))
		w.WriteHeader(http.StatusNotFound)
	}))
	result.TestHttpServer = httpServer
	return result
}

type TestAzureKeyVault struct {
	keys           map[string]crypto.PrivateKey
	certificates   map[string]*tls.Certificate
	TestHttpServer *httptest.Server
}

func (t TestAzureKeyVault) AddCertificate(name string, cert *tls.Certificate) {
	t.certificates[name] = cert
	t.keys[name] = cert.PrivateKey
}

func (t TestAzureKeyVault) AddKey(name string, key *rsa.PrivateKey) {
	t.keys[name] = key
}

func (t TestAzureKeyVault) keyToBundle(name string, key crypto.PrivateKey) azkeys.KeyBundle {
	id := azkeys.ID(t.TestHttpServer.URL + "/keys/" + name + "/0")
	switch key := key.(type) {
	case *rsa.PrivateKey:
		e := make([]byte, 8)
		binary.BigEndian.PutUint64(e, uint64(key.E))
		return azkeys.KeyBundle{
			Key: &azkeys.JSONWebKey{
				Kty: to.Ptr(azkeys.KeyTypeRSA),
				E:   e,
				N:   key.N.Bytes(),
				KID: &id,
			},
		}
	case *ecdsa.PrivateKey:
		var crv azkeys.CurveName
		switch key.Curve.Params().Name {
		case elliptic.P256().Params().Name:
			crv = azkeys.CurveNameP256
		default:
			panic(fmt.Errorf("unsupported curve: %s", key.Curve.Params().Name).Error())
		}
		return azkeys.KeyBundle{
			Key: &azkeys.JSONWebKey{
				Kty: to.Ptr(azkeys.KeyTypeEC),
				Crv: to.Ptr(crv),
				X:   key.X.Bytes(),
				Y:   key.Y.Bytes(),
				KID: &id,
			},
		}
	default:
		panic(fmt.Errorf("unsupported key type: %T", key).Error())
	}
}
