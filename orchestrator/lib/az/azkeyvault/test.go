package azkeyvault

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"net/http/httptest"
)

func NewTestServer() *TestAzureKeyVault {
	result := &TestAzureKeyVault{
		keys:         make(map[string]*rsa.PrivateKey),
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
	httpServerMux.Handle("POST /keys/{name}/0/decrypt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBytes, _ := io.ReadAll(r.Body)
		name := r.PathValue("name")
		println("data " + string(requestBytes))
		w.WriteHeader(http.StatusOK)
		return
		if key, ok := result.keys[name]; ok {
			plainText, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, key, []byte("test"), nil)
			if err != nil {
				log.Logger.Err(err).Msg("failed to decrypt")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var response = azkeys.DecryptResponse{
				KeyOperationResult: azkeys.KeyOperationResult{
					IV:     nil,
					KID:    nil,
					Result: plainText,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
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
		log.Logger.Error().Msgf("unhandled request: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	result.TestHttpServer = httpServer
	return result
}

type TestAzureKeyVault struct {
	keys           map[string]*rsa.PrivateKey
	certificates   map[string]*tls.Certificate
	TestHttpServer *httptest.Server
}

func (t TestAzureKeyVault) AddCertificate(name string, cert *tls.Certificate) {
	t.certificates[name] = cert
}

func (t TestAzureKeyVault) AddKey(name string, key *rsa.PrivateKey) {
	t.keys[name] = key
}

func (t TestAzureKeyVault) keyToBundle(name string, key *rsa.PrivateKey) azkeys.KeyBundle {
	id := azkeys.ID(t.TestHttpServer.URL + "/keys/" + name + "/0")
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
}
