package nuts

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/SanteonNL/orca/orchestrator/lib/test/vcrclient_mock"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	ssi "github.com/nuts-foundation/go-did"
	"github.com/nuts-foundation/go-did/vc"
	"github.com/nuts-foundation/go-nuts-client/nuts/vcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDutchNutsProfile_identifiersFromCredential(t *testing.T) {
	t.Run("NutsUraCredential", func(t *testing.T) {
		uraCredential, err := vc.ParseVerifiableCredential(`{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://nuts.nl/credentials/2024",
    "https://w3c-ccg.github.io/lds-jws2020/contexts/lds-jws2020-v1.json"
  ],
  "credentialSubject": {
    "id": "did:web:h5tcgbxz-7080.euw.devtunnels.ms:iam:dd647db6-1fd1-4ea1-9691-75041f3d66ea",
    "organization": {
      "city": "Utrecht",
      "name": "Demo Clinic",
      "ura": "1234"
    }
  },
  "id": "did:web:h5tcgbxz-7080.euw.devtunnels.ms:iam:dd647db6-1fd1-4ea1-9691-75041f3d66ea#685f2ddf-fb1f-4618-bdcd-c2c16dd24646",
  "issuanceDate": "2024-07-17T11:30:48.441027305Z",
  "issuer": "did:web:h5tcgbxz-7080.euw.devtunnels.ms:iam:dd647db6-1fd1-4ea1-9691-75041f3d66ea",
  "proof": {
    "created": "2024-07-17T11:30:48.441027305Z",
    "jws": "eyJhbGciOiJFUzI1NiIsImI2NCI6ZmFsc2UsImNyaXQiOlsiYjY0Il0sImtpZCI6ImRpZDp3ZWI6aDV0Y2dieHotNzA4MC5ldXcuZGV2dHVubmVscy5tczppYW06ZGQ2NDdkYjYtMWZkMS00ZWExLTk2OTEtNzUwNDFmM2Q2NmVhIzAifQ..E3XI1uSdMjBr4-7FRn5QZdhUvy92KY9PRlH-ia_x8V5Isvg7Ol0BXFlv60DQbWszLg_TdhE91jfSz_BOPHWgqQ",
    "proofPurpose": "assertionMethod",
    "type": "JsonWebSignature2020",
    "verificationMethod": "did:web:h5tcgbxz-7080.euw.devtunnels.ms:iam:dd647db6-1fd1-4ea1-9691-75041f3d66ea#0"
  },
  "type": [
    "NutsUraCredential",
    "VerifiableCredential"
  ]
}`)
		require.NoError(t, err)

		profile := &DutchNutsProfile{}
		identifiers, err := profile.identifiersFromCredential(*uraCredential)

		require.NoError(t, err)
		assert.Len(t, identifiers, 1)
		assert.Equal(t, coolfhir.URANamingSystem, *identifiers[0].Identifier[0].System)
		assert.Equal(t, "1234", *identifiers[0].Identifier[0].Value)
		assert.Equal(t, "Demo Clinic", *identifiers[0].Name)
	})
	t.Run("X509Credential", func(t *testing.T) {
		credential, err := vc.ParseVerifiableCredential("eyJhbGciOiJQUzI1NiIsImtpZCI6ImRpZDp4NTA5OjA6c2hhMjU2OnN6cU1hVHBuRDZHTjBhUnJUOThlVjRiaEFvT2d5SXRFWlZ5c2tZeUxfUWM6OnNhbjpvdGhlck5hbWU6Mi4xNi41MjguMS4xMDA3Ljk5LjIxMTAtMS04NjQ0Ni1TLTg2NDQ2LTAwLjAwMC04NjQ0Njo6c3ViamVjdDpPOlpvcmclMjBiaWolMjBqb3UlMjBCLlYuOjpzdWJqZWN0Okw6VXRyZWNodCMwIiwidHlwIjoiSldUIiwieDVjIjpbIk1JSUR5VENDQXJHZ0F3SUJBZ0lVRlRQTytwVWszMlFXc1l5TFlkbExUbWxSV1V3d0RRWUpLb1pJaHZjTkFRRUxCUUF3R3pFWk1CY0dBMVVFQXd3UVJtRnJaU0JWV2trZ1VtOXZkQ0JEUVRBZUZ3MHlOREV4TVRVd09ESXdNVGxhRncweU5URXhNVFV3T0RJd01UbGFNSE14TXpBeEJnTlZCQU1NS25wdmNtZGlhV3BxYjNVdWRHVnpkQzVwYm5SbFozSmhkR2x2Ymk1NmIzSm5ZbWxxYW05MUxtTnZiVEVhTUJnR0ExVUVDZ3dSV205eVp5QmlhV29nYW05MUlFSXVWaTR4RURBT0JnTlZCQWNNQjFWMGNtVmphSFF4RGpBTUJnTlZCQVVUQlRnMk5EUTJNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZ0tDQVFFQXpaZVlrNW0zbWx1Vjl1QnNQMUc2amkyYWdlaEhaUElSOEtNSWNuMjgyN3BUN3hFdG1wb3FZUDloMVRiVXJyM2Z5anN1TW9oNzFucnVhMzM3dTBrYlpJaC9RZVdhQllXUXV4VStJUjNRdDJMM05IY2NNRmk5M05pMXcxLzV3c1NUMEw4dXBnNG9FNGhZR3ROa2RyZ1pHK3Q4Z0Y2RGp4b0E4SW4yRDN1cW1waldFcUgzUytPSU1nTVlHNTRxeTl6UStFNW9penFyUzhjLzdzVW8ydFFFYk1VUXFHWnI0UjBxRzJOcUNLSzdIWWFFT2tHd2YwL1ZZVXRIR3ZZdFhaM0s2eWkzRUxwRDRYSVdXdVJ0UjNldmpERHpYM1M1a3R2bFowcUI5WTUzWm10VnNMejd3YXZUUHNaUXp0NmNvYjJkL21LdVBnYmZwYlRja1JCY0trZmJ4UUlEQVFBQm80R3NNSUdwTUIwR0ExVWRKUVFXTUJRR0NDc0dBUVVGQndNQkJnZ3JCZ0VGQlFjREFqQklCZ05WSFJFRVFUQS9vRDBHQTFVRkJhQTJERFF5TGpFMkxqVXlPQzR4TGpFd01EY3VPVGt1TWpFeE1DMHhMVGcyTkRRMkxWTXRPRFkwTkRZdE1EQXVNREF3TFRnMk5EUTJNQjBHQTFVZERnUVdCQlIxRkdaMjJyWUE4bWlaUXVLUmNjRVNBWWVkSnpBZkJnTlZIU01FR0RBV2dCU2JzYzlGOERlejNYakljaVgzSDV1NmN0UVN2VEFOQmdrcWhraUc5dzBCQVFzRkFBT0NBUUVBWktSR1M1UE5MRDkwNkd3cHF3Z3hTaVU4amNkcXhjdmRNR2NiMmZXRDZSZFI0NG81S2JhVUZvdlp4Y0N3M21uSEo1TXZYY0ppeS9hYlUrdWFmcjBIY1VHY0JaZWZIQnFkWFdhSUMweTlnUklhaFF2K3paajI4SFlFUEpzRVpXeldRQ1JQZ3QrRGhuZlV2VC8zV3o1R0duY054Rk1FVEFVL0NaZzg0cVgxVGNseFFtZXdtSnpTdi96VFBKRlZYdzA2TTdYUHl2dFhRRUtmMTNINnpKalJGbWlrTStrSzkySFdIRmF3dmRXVWpDbTJEc3ZBYTlON0xERUhnMW55Q2dKTVJjVjV3UVV6VEFwUHhaSFcvOTZhYTJGZFkzaDU0UkpDQ3RNOENZa1dLWk84OW1XQjFCdUI4bG1Ld2g5c2xUM3lDTU4yL2hFRFRtN0p4V3p3MWdmNy93PT0iLCJNSUlDOWpDQ0FkNmdBd0lCQWdJVVJGQ3FQckwzUVFkQk5PcWt3bVhXTmd4OXBkUXdEUVlKS29aSWh2Y05BUUVMQlFBd0d6RVpNQmNHQTFVRUF3d1FSbUZyWlNCVldra2dVbTl2ZENCRFFUQWVGdzB5TkRFeE1URXhOREUxTVRoYUZ3MHpOREV4TURreE5ERTFNVGhhTUJzeEdUQVhCZ05WQkFNTUVFWmhhMlVnVlZwSklGSnZiM1FnUTBFd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUURUNUo4Z0tkeU1KTmkzY3VBbUorTUlMck11d3JLeVRSWWhqVVVGSEhuNXJjVmFITjBoekI2djV0NzROdDQweFVYUk5hb21EY2NsQklPbHd0OGY2MkpBMnAvajgzRU5mZExyWHZVdTlOTVRoa3Fad1o5ZHpSd0s3bDNVWkJxOE5UUVVPNzRXNE0ycXg4bnJYcTMxZVdvZ3hVVUlGYzFYT1JoNWVjZWJlTDVtVWIyRTZVbG1EbU5nbTJmR2VTbW1pczh6aWVJK0tLWU9oaS9oWXR5ZWl4cmc3cnhQNHYwVlJyRXN0Y1dBZXRSZ1hXUVgwRWxBeHMwVnJzeTYvdnYzcEV0WGh4OHdiMndpMnhZMTRkOUloOEhkZU5JKyszd0liWno2V1ZNM2ZENVFGSFYyRVpCSCtzb28wcGZLajJ0SHNhRHozRlBNdU16SUx0NlU2UFQ0QUxJZEFnTUJBQUdqTWpBd01BOEdBMVVkRXdRSU1BWUJBZjhDQVFBd0hRWURWUjBPQkJZRUZKdXh6MFh3TjdQZGVNaHlKZmNmbTdweTFCSzlNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUFobHBrejY4eDJkR3BPTFgzRnpBYjhFZStZMk9WK1JXRnBzTUU5WlZEVTA2SkVUUGZQQ2owMlBIODJsZ1VuYzRqZVI4MXJQU3NJdDJzc3FtMlM0emIwMk5pcDU5NWNBcUNLdm1CZkVjOWhQUFcydWdwTnhUOFpSVTRMS3JxcFY0bko2bkJ2RHFtR3VINXVxOU5nOWw5U25NM2VLbWRadEpLYytaTkFQS3hWQWl1ZUxUZHI2VzJVYm1Lb1pBUlFRMEpMa0ZuWk94blVrcjhwUWZ4VXpFSVVrSGcyZFdhYUkvNHdvNFBuaTd4WGdnRm9QRHBWenR1L2lQMzNYQkxxWEp3eHhIWGhxOW5jOUpVL2tFWER0N2o4RWdveUpvN0pqU0tjanBSZnBHa0U1Z3FxQjRTYTh3QXNBUFVLM2pScmV1eXRsbEF0UVVaUmJDdEhieGNsYzl5QSJdLCJ4NXQiOiJybnFhd2RoUXZ1bl9pa3JzcFBUaHpqblg2NEEifQ.eyJleHAiOjE3NjMxOTQ4MTksImlzcyI6ImRpZDp4NTA5OjA6c2hhMjU2OnN6cU1hVHBuRDZHTjBhUnJUOThlVjRiaEFvT2d5SXRFWlZ5c2tZeUxfUWM6OnNhbjpvdGhlck5hbWU6Mi4xNi41MjguMS4xMDA3Ljk5LjIxMTAtMS04NjQ0Ni1TLTg2NDQ2LTAwLjAwMC04NjQ0Njo6c3ViamVjdDpPOlpvcmclMjBiaWolMjBqb3UlMjBCLlYuOjpzdWJqZWN0Okw6VXRyZWNodCIsImp0aSI6ImRpZDp4NTA5OjA6c2hhMjU2OnN6cU1hVHBuRDZHTjBhUnJUOThlVjRiaEFvT2d5SXRFWlZ5c2tZeUxfUWM6OnNhbjpvdGhlck5hbWU6Mi4xNi41MjguMS4xMDA3Ljk5LjIxMTAtMS04NjQ0Ni1TLTg2NDQ2LTAwLjAwMC04NjQ0Njo6c3ViamVjdDpPOlpvcmclMjBiaWolMjBqb3UlMjBCLlYuOjpzdWJqZWN0Okw6VXRyZWNodCMzNGNiMjMyMC01NjU4LTQzN2MtOWVjOS00ZTQ2YjZkZGY5YTQiLCJuYmYiOjE3Mzc1MjY4MDYsInN1YiI6ImRpZDp3ZWI6em9yZ3BsYXRmb3JtLnRlc3QuaW50ZWdyYXRpb24uem9yZ2JpampvdS5jb206bnV0czppYW06OTI5YWI1MmUtMzY1OS00YzIxLTg5YzUtODRiZWZmMTIzMzc1IiwidmMiOnsiQGNvbnRleHQiOlsiaHR0cHM6Ly93d3cudzMub3JnLzIwMTgvY3JlZGVudGlhbHMvdjEiXSwiY3JlZGVudGlhbFN1YmplY3QiOlt7ImlkIjoiZGlkOndlYjp6b3JncGxhdGZvcm0udGVzdC5pbnRlZ3JhdGlvbi56b3JnYmlqam91LmNvbTpudXRzOmlhbTo5MjlhYjUyZS0zNjU5LTRjMjEtODljNS04NGJlZmYxMjMzNzUiLCJzYW4iOnsib3RoZXJOYW1lIjoiMi4xNi41MjguMS4xMDA3Ljk5LjIxMTAtMS04NjQ0Ni1TLTg2NDQ2LTAwLjAwMC04NjQ0NiJ9LCJzdWJqZWN0Ijp7IkwiOiJVdHJlY2h0IiwiTyI6IlpvcmcgYmlqIGpvdSBCLlYuIn19XSwidHlwZSI6WyJWZXJpZmlhYmxlQ3JlZGVudGlhbCIsIlg1MDlDcmVkZW50aWFsIl19fQ.e8SWk478368PrXoXTQrjbEOGfQj13LS2nTzJ8n_wL7xCZ7JsAmWwG9LCPXfLReYZlA3qu81pxgCktmoePcvvF7QWH39ajnhxr7UQdy3NZkZzmFehdweM4pDvyQFpiJrwR3urHh_JzPF0y0jbnagZzH4n9ZPAqiNwWjVMuTig3tjOO8MIrqhRGLrj1zpKIy1J_oMvwx7X91FYPjRxdztn9rB6Tf4Z5BoKsDzHKGZXU3bKhA1rllknvD6jPB00rYQbAFkASz052A3YDmMAIdgfEbO75Rcx9TfRxuaXH1ok1-RZbscHlcguFcHIddeUWe7Wru-cN1PqsUElEBkJqG9K5g")
		require.NoError(t, err)

		profile := &DutchNutsProfile{}
		identifiers, err := profile.identifiersFromCredential(*credential)

		require.NoError(t, err)
		assert.Len(t, identifiers, 1)
		assert.Equal(t, coolfhir.URANamingSystem, *identifiers[0].Identifier[0].System)
		assert.Equal(t, "86446", *identifiers[0].Identifier[0].Value)
		assert.Equal(t, "Zorg bij jou B.V.", *identifiers[0].Name)
	})

}

func TestDutchNutsProfile_Identities(t *testing.T) {
	ctx := context.Background()
	identifier1 := fhir.Identifier{
		System: to.Ptr(coolfhir.URANamingSystem),
		Value:  to.Ptr("1234"),
	}
	identity1 := fhir.Organization{
		Identifier: []fhir.Identifier{identifier1},
		Name:       to.Ptr("Demo Clinic"),
	}
	identifier1VC := vc.VerifiableCredential{
		Type: []ssi.URI{ssi.MustParseURI("NutsUraCredential")},
		CredentialSubject: []interface{}{
			map[string]interface{}{
				"organization": map[string]interface{}{
					"ura":  *identifier1.Value,
					"name": *identity1.Name,
				},
			},
		},
	}
	identifier2 := fhir.Identifier{
		System: to.Ptr(coolfhir.URANamingSystem),
		Value:  to.Ptr("5678"),
	}
	identity2 := fhir.Organization{
		Identifier: []fhir.Identifier{identifier2},
		// This care organization doesn't have a name
	}
	identifier2VC := vc.VerifiableCredential{
		Type: []ssi.URI{ssi.MustParseURI("NutsUraCredential")},
		CredentialSubject: []interface{}{
			map[string]interface{}{
				"organization": map[string]interface{}{
					"ura": *identifier2.Value,
				},
			},
		},
	}
	identifier2UziCertVC := vc.VerifiableCredential{
		Type: []ssi.URI{ssi.MustParseURI("X509Credential")},
		CredentialSubject: []interface{}{
			map[string]interface{}{
				"san": map[string]interface{}{
					"otherName": "2.16.528.1.1007.99.2110-1-111-S-" + *identifier2.Value + "-00.000-222",
				},
				"subject": map[string]interface{}{
					"O": "Demo Clinic",
				},
			},
		},
	}
	nonUraVC := vc.VerifiableCredential{
		CredentialSubject: []interface{}{
			map[string]interface{}{
				"name": "test",
			},
		},
	}
	problemResponse := &vcr.GetCredentialsInWalletResponse{
		ApplicationproblemJSONDefault: &struct {
			Detail string  `json:"detail"`
			Status float32 `json:"status"`
			Title  string  `json:"title"`
		}{
			Detail: "something went wrong",
			Status: http.StatusInternalServerError,
			Title:  "Oops",
		},
	}
	t.Run("UziServerCertificateCredential in wallet", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		vcrClient := vcrclient_mock.NewMockClientWithResponsesInterface(ctrl)
		prof := &DutchNutsProfile{
			vcrClient: vcrClient,
			Config:    Config{OwnSubject: "sub"},
		}
		vcrClient.EXPECT().GetCredentialsInWalletWithResponse(ctx, "sub").Return(&vcr.GetCredentialsInWalletResponse{
			JSON200: &[]vcr.VerifiableCredential{identifier1VC, identifier2UziCertVC},
		}, nil)

		identities, err := prof.Identities(ctx)
		require.NoError(t, err)

		require.Len(t, identities, 2)
		assert.Equal(t, identities[0].Identifier[0], identity1.Identifier[0])
		assert.Equal(t, identities[1].Identifier[0], identity2.Identifier[0])
	})
	t.Run("NutsUraCredential and others in wallet", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		vcrClient := vcrclient_mock.NewMockClientWithResponsesInterface(ctrl)
		prof := &DutchNutsProfile{
			vcrClient: vcrClient,
			Config:    Config{OwnSubject: "sub"},
		}
		vcrClient.EXPECT().GetCredentialsInWalletWithResponse(ctx, "sub").Return(&vcr.GetCredentialsInWalletResponse{
			JSON200: &[]vcr.VerifiableCredential{identifier1VC, nonUraVC},
		}, nil)

		identities, err := prof.Identities(ctx)
		require.NoError(t, err)

		require.Len(t, identities, 1)
		assert.Equal(t, identities[0].Identifier[0], identity1.Identifier[0])
	})
	t.Run("initial fetch, then cached", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		vcrClient := vcrclient_mock.NewMockClientWithResponsesInterface(ctrl)
		prof := &DutchNutsProfile{
			vcrClient: vcrClient,
			Config:    Config{OwnSubject: "sub"},
		}
		vcrClient.EXPECT().GetCredentialsInWalletWithResponse(ctx, "sub").Return(&vcr.GetCredentialsInWalletResponse{
			JSON200: &[]vcr.VerifiableCredential{identifier1VC, identifier2VC},
		}, nil)

		identities, err := prof.Identities(ctx)
		require.NoError(t, err)

		require.Len(t, identities, 2)
		assert.Equal(t, identities[0].Identifier[0], identity1.Identifier[0])
		assert.Equal(t, identities[1].Identifier[0], identity2.Identifier[0])

		identities, err = prof.Identities(ctx)
		require.NoError(t, err)
		require.Len(t, identities, 2)
	})
	t.Run("initial fetch fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		vcrClient := vcrclient_mock.NewMockClientWithResponsesInterface(ctrl)
		prof := &DutchNutsProfile{
			vcrClient: vcrClient,
			Config:    Config{OwnSubject: "sub"},
		}
		vcrClient.EXPECT().GetCredentialsInWalletWithResponse(ctx, "sub").Return(nil, errors.New("failed"))

		identities, err := prof.Identities(ctx)
		require.EqualError(t, err, "failed to load local identities: failed to list credentials: failed")
		require.Nil(t, identities)
	})
	t.Run("initial fetch fails with Problem", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		vcrClient := vcrclient_mock.NewMockClientWithResponsesInterface(ctrl)
		prof := &DutchNutsProfile{
			vcrClient: vcrClient,
			Config:    Config{OwnSubject: "sub"},
		}
		vcrClient.EXPECT().GetCredentialsInWalletWithResponse(ctx, "sub").Return(problemResponse, nil)

		identities, err := prof.Identities(ctx)
		require.EqualError(t, err, "failed to load local identities: list credentials non-OK HTTP response (status=): HTTP 500 - Oops - something went wrong")
		require.Nil(t, identities)
	})
	t.Run("(stale) cache is returned if refresh fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		vcrClient := vcrclient_mock.NewMockClientWithResponsesInterface(ctrl)
		prof := &DutchNutsProfile{
			vcrClient: vcrClient,
			Config:    Config{OwnSubject: "sub"},
			cachedIdentities: []fhir.Organization{
				{
					Identifier: []fhir.Identifier{identifier1},
				},
			},
		}
		vcrClient.EXPECT().GetCredentialsInWalletWithResponse(ctx, "sub").Return(nil, errors.New("failed"))

		identities, err := prof.Identities(ctx)
		require.NoError(t, err)
		require.Len(t, identities, 1)
		assert.Equal(t, identities[0].Identifier[0], identity1.Identifier[0])
	})
	t.Run("fetched again when cache is expired", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		vcrClient := vcrclient_mock.NewMockClientWithResponsesInterface(ctrl)
		prof := &DutchNutsProfile{
			vcrClient: vcrClient,
			Config:    Config{OwnSubject: "sub"},
		}
		vcrClient.EXPECT().GetCredentialsInWalletWithResponse(ctx, "sub").Return(&vcr.GetCredentialsInWalletResponse{
			JSON200: &[]vcr.VerifiableCredential{identifier1VC, identifier2VC},
		}, nil).Times(2)

		identities, err := prof.Identities(ctx)
		require.NoError(t, err)

		require.Len(t, identities, 2)
		assert.Contains(t, identities, identity1)
		assert.Contains(t, identities, identity2)

		// expire cache
		prof.identitiesRefreshedAt = prof.identitiesRefreshedAt.Add(-identitiesCacheTTL)

		identities, err = prof.Identities(ctx)
		require.NoError(t, err)
		require.Len(t, identities, 2)
	})
}

func TestDutchNutsProfile_HttpClient(t *testing.T) {
	serverIdentity := fhir.Identifier{System: to.Ptr(coolfhir.URANamingSystem), Value: to.Ptr("1234")}
	ctrl := gomock.NewController(t)
	mockCSD := csd.NewMockDirectory(ctrl)
	mockCSD.EXPECT().LookupEndpoint(gomock.Any(), &serverIdentity, authzServerURLEndpointName).
		Return([]fhir.Endpoint{
			{
				Address: "https://example.com/authz",
			},
		}, nil).
		AnyTimes()
	ctx := context.Background()
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/internal/auth/v2/sub/request-service-access-token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "token",
			"token_type":   "Bearer",
		})
	})
	capturedHeaders := make(http.Header)
	httpMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	})

	t.Run("without client cert", func(t *testing.T) {
		t.Cleanup(func() {
			capturedHeaders = nil
		})
		httpServer := httptest.NewServer(httpMux)
		defer httpServer.Close()

		profile := DutchNutsProfile{
			csd: mockCSD,
			Config: Config{
				API: APIConfig{
					URL: httpServer.URL,
				},
				OwnSubject: "sub",
			},
		}
		httpClient, err := profile.HttpClient(ctx, serverIdentity)
		require.NoError(t, err)
		httpResponse, err := httpClient.Get(httpServer.URL)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		require.Equal(t, "Bearer token", capturedHeaders.Get("Authorization"))
	})
	t.Run("with client cert", func(t *testing.T) {
		t.Cleanup(func() {
			capturedHeaders = nil
		})
		// Create an HTTP test server that requires a TLS client certificate
		nutsNode := httptest.NewServer(httpMux)
		// Create an HTTP test server that requires a TLS client certificate
		resourceServer := httptest.NewUnstartedServer(httpMux)
		defer resourceServer.Close()
		clientCert, err := tls.LoadX509KeyPair("test_cert.pem", "test_cert_key.pem")
		require.NoError(t, err)
		resourceServer.TLS = &tls.Config{}
		resourceServer.TLS.ClientCAs = x509.NewCertPool()
		resourceServer.TLS.ClientCAs.AppendCertsFromPEM(clientCert.Certificate[0])
		resourceServer.TLS.ClientAuth = tls.RequireAnyClientCert
		resourceServer.StartTLS()

		globals.DefaultTLSConfig = resourceServer.Client().Transport.(*http.Transport).TLSClientConfig
		profile := DutchNutsProfile{
			clientCerts: []tls.Certificate{clientCert},
			csd:         mockCSD,
			Config: Config{
				API: APIConfig{
					URL: nutsNode.URL,
				},
				OwnSubject: "sub",
			},
		}
		httpClient, err := profile.HttpClient(ctx, serverIdentity)
		require.NoError(t, err)
		httpResponse, err := httpClient.Get(resourceServer.URL)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		require.Equal(t, "Bearer token", capturedHeaders.Get("Authorization"))
	})
	t.Run("FHIR base URL as identity, AuthorizationServer URL lookup using CapabilityStatement", func(t *testing.T) {
		var fhirBaseURL string
		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			coolfhir.SendResponse(w, 200, fhir.CapabilityStatement{
				Rest: []fhir.CapabilityStatementRest{
					{
						Security: &fhir.CapabilityStatementRestSecurity{
							Service: []fhir.CodeableConcept{
								{
									Coding: []fhir.Coding{
										{
											System: to.Ptr("http://hl7.org/fhir/ValueSet/restful-security-service"),
											Code:   to.Ptr("OAuth"),
										},
									},
									Extension: []fhir.Extension{
										{
											Url:         "http://santeonnl.github.io/shared-care-planning/StructureDefinition/Nuts#AuthorizationServer",
											ValueString: to.Ptr(fhirBaseURL + "/authz"),
										},
									},
								},
							},
						},
					},
				},
			})
		}))
		fhirBaseURL = httpServer.URL

		serverIdentity := fhir.Identifier{
			System: to.Ptr("https://build.fhir.org/http.html#root"),
			Value:  to.Ptr(fhirBaseURL),
		}
		profile := DutchNutsProfile{}
		httpClient, err := profile.HttpClient(ctx, serverIdentity)
		require.NoError(t, err)
		require.NotNil(t, httpClient)
	})
}

func TestNew(t *testing.T) {
	t.Run("load client certificate from Azure Key Vault", func(t *testing.T) {
		kv := azkeyvault.NewTestServer()
		azkeyvault.AzureHttpRequestDoer = kv.TestHttpServer.Client()
		cert, err := tls.LoadX509KeyPair("test_cert.pem", "test_cert_key.pem")
		require.NoError(t, err)
		kv.AddCertificate("test-client-cert", &cert)
		kv.AddCertificate("test-client-cert-2", &cert)

		profile, err := New(Config{
			AzureKeyVault: AzureKeyVaultConfig{
				URL:            kv.TestHttpServer.URL,
				ClientCertName: []string{"test-client-cert", "test-client-cert-2"},
			},
		})
		require.NoError(t, err)
		require.Len(t, profile.clientCerts, 2)
	})
}

func TestDutchNutsProfile_CapabilityStatement(t *testing.T) {
	md := fhir.CapabilityStatement{
		Rest: []fhir.CapabilityStatementRest{
			{
				Mode: fhir.RestfulCapabilityModeServer,
			},
		},
	}
	profile := DutchNutsProfile{
		Config: Config{
			Public:     PublicConfig{URL: "https://example.com"},
			OwnSubject: "sub",
		},
	}
	profile.CapabilityStatement(&md)
	actual, err := json.Marshal(md)
	require.NoError(t, err)
	expected := `
{
  "status": "draft",
  "date": "",
  "kind": "instance",
  "fhirVersion": "0.01",
  "format": null,
  "rest": [
    {
      "mode": "server",
      "security": {
        "service": [
          {
            "extension": [
              {
                "url": "http://santeonnl.github.io/shared-care-planning/StructureDefinition/Nuts#AuthorizationServer",
                "valueString": "https://example.com/oauth2/sub"
              }
            ],
            "coding": [
              {
                "system": "http://hl7.org/fhir/ValueSet/restful-security-service",
                "code": "OAuth"
              }
            ]
          }
        ]
      }
    }
  ],
  "resourceType": "CapabilityStatement"
}`
	assert.JSONEq(t, expected, string(actual))
}
