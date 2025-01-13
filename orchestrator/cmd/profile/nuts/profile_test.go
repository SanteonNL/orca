package nuts

import (
	"context"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test/vcrclient_mock"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	ssi "github.com/nuts-foundation/go-did"
	"github.com/nuts-foundation/go-did/vc"
	"github.com/nuts-foundation/go-nuts-client/nuts/vcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDutchNutsProfile_RegisterHTTPHandlers(t *testing.T) {
	const basePath = "/basement"
	var baseURL, _ = url.Parse("http://example.com" + basePath)
	serverMux := http.NewServeMux()
	DutchNutsProfile{}.RegisterHTTPHandlers("/basement", baseURL, serverMux)
	server := httptest.NewServer(serverMux)

	httpResponse, err := http.Get(server.URL + "/basement/.well-known/oauth-protected-resource")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	data, _ := io.ReadAll(httpResponse.Body)
	assert.JSONEq(t, `{"resource":"http://example.com/basement","authorization_servers":["oauth2"],"bearer_methods_supported":["header"]}`, string(data))
}

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
		identities, err := profile.identifiersFromCredential(*uraCredential)

		require.NoError(t, err)
		assert.Len(t, identities, 1)
		assert.Len(t, identities[0].Identifier, 1)
		assert.Equal(t, coolfhir.URANamingSystem, *identities[0].Identifier[0].System)
		assert.Equal(t, "1234", *identities[0].Identifier[0].Value)
		assert.Equal(t, "Demo Clinic", *identities[0].Name)
		assert.Equal(t, "Utrecht", *identities[0].Address[0].City)
	})
	t.Run("UziServerCertificateCredential", func(t *testing.T) {
		credential, err := vc.ParseVerifiableCredential("eyJhbGciOiJQUzUxMiIsImtpZCI6ImRpZDp4NTA5OjA6c2hhNTEyOkN0Z0FaZENpSEtsSmNIb1FOWkpod1dDSUN6ZS1EM2R1TzY1cDk1cWJfSDlxVTAtNVUzdXhESWpsR1p3S1ZYeXpBcEdRWWF1Q1oxUlFXZ2p6YWdMYWNROjpzYW46b3RoZXJOYW1lOjIuMTYuNTI4LjEuMTAwNy45OS4yMTEwLTEtODY0NDYtUy04NjQ0Ni0wMC4wMDAtODY0NDY6OnN1YmplY3Q6Tzpab3JnJTIwYmlqJTIwam91JTIwQi5WLjo6c3ViamVjdDpMOlV0cmVjaHQjMCIsInR5cCI6IkpXVCIsIng1YyI6WyItLS0tLUJFR0lOIENFUlRJRklDQVRFLS0tLS1cbk1JSUR5VENDQXJHZ0F3SUJBZ0lVRlRQTytwVWszMlFXc1l5TFlkbExUbWxSV1V3d0RRWUpLb1pJaHZjTkFRRUxcbkJRQXdHekVaTUJjR0ExVUVBd3dRUm1GclpTQlZXa2tnVW05dmRDQkRRVEFlRncweU5ERXhNVFV3T0RJd01UbGFcbkZ3MHlOVEV4TVRVd09ESXdNVGxhTUhNeE16QXhCZ05WQkFNTUtucHZjbWRpYVdwcWIzVXVkR1Z6ZEM1cGJuUmxcblozSmhkR2x2Ymk1NmIzSm5ZbWxxYW05MUxtTnZiVEVhTUJnR0ExVUVDZ3dSV205eVp5QmlhV29nYW05MUlFSXVcblZpNHhFREFPQmdOVkJBY01CMVYwY21WamFIUXhEakFNQmdOVkJBVVRCVGcyTkRRMk1JSUJJakFOQmdrcWhraUdcbjl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUF6WmVZazVtM21sdVY5dUJzUDFHNmppMmFnZWhIWlBJUjhLTUlcbmNuMjgyN3BUN3hFdG1wb3FZUDloMVRiVXJyM2Z5anN1TW9oNzFucnVhMzM3dTBrYlpJaC9RZVdhQllXUXV4VStcbklSM1F0MkwzTkhjY01GaTkzTmkxdzEvNXdzU1QwTDh1cGc0b0U0aFlHdE5rZHJnWkcrdDhnRjZEanhvQThJbjJcbkQzdXFtcGpXRXFIM1MrT0lNZ01ZRzU0cXk5elErRTVvaXpxclM4Yy83c1VvMnRRRWJNVVFxR1pyNFIwcUcyTnFcbkNLSzdIWWFFT2tHd2YwL1ZZVXRIR3ZZdFhaM0s2eWkzRUxwRDRYSVdXdVJ0UjNldmpERHpYM1M1a3R2bFowcUJcbjlZNTNabXRWc0x6N3dhdlRQc1pRenQ2Y29iMmQvbUt1UGdiZnBiVGNrUkJjS2tmYnhRSURBUUFCbzRHc01JR3Bcbk1CMEdBMVVkSlFRV01CUUdDQ3NHQVFVRkJ3TUJCZ2dyQmdFRkJRY0RBakJJQmdOVkhSRUVRVEEvb0QwR0ExVUZcbkJhQTJERFF5TGpFMkxqVXlPQzR4TGpFd01EY3VPVGt1TWpFeE1DMHhMVGcyTkRRMkxWTXRPRFkwTkRZdE1EQXVcbk1EQXdMVGcyTkRRMk1CMEdBMVVkRGdRV0JCUjFGR1oyMnJZQThtaVpRdUtSY2NFU0FZZWRKekFmQmdOVkhTTUVcbkdEQVdnQlNic2M5RjhEZXozWGpJY2lYM0g1dTZjdFFTdlRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQVpLUkdcblM1UE5MRDkwNkd3cHF3Z3hTaVU4amNkcXhjdmRNR2NiMmZXRDZSZFI0NG81S2JhVUZvdlp4Y0N3M21uSEo1TXZcblhjSml5L2FiVSt1YWZyMEhjVUdjQlplZkhCcWRYV2FJQzB5OWdSSWFoUXYrelpqMjhIWUVQSnNFWld6V1FDUlBcbmd0K0RobmZVdlQvM1d6NUdHbmNOeEZNRVRBVS9DWmc4NHFYMVRjbHhRbWV3bUp6U3YvelRQSkZWWHcwNk03WFBcbnl2dFhRRUtmMTNINnpKalJGbWlrTStrSzkySFdIRmF3dmRXVWpDbTJEc3ZBYTlON0xERUhnMW55Q2dKTVJjVjVcbndRVXpUQXBQeFpIVy85NmFhMkZkWTNoNTRSSkNDdE04Q1lrV0taTzg5bVdCMUJ1QjhsbUt3aDlzbFQzeUNNTjJcbi9oRURUbTdKeFd6dzFnZjcvdz09XG4tLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tIiwiLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tXG5NSUlDOWpDQ0FkNmdBd0lCQWdJVVJGQ3FQckwzUVFkQk5PcWt3bVhXTmd4OXBkUXdEUVlKS29aSWh2Y05BUUVMXG5CUUF3R3pFWk1CY0dBMVVFQXd3UVJtRnJaU0JWV2trZ1VtOXZkQ0JEUVRBZUZ3MHlOREV4TVRFeE5ERTFNVGhhXG5GdzB6TkRFeE1Ea3hOREUxTVRoYU1Cc3hHVEFYQmdOVkJBTU1FRVpoYTJVZ1ZWcEpJRkp2YjNRZ1EwRXdnZ0VpXG5NQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUURUNUo4Z0tkeU1KTmkzY3VBbUorTUlMck11XG53ckt5VFJZaGpVVUZISG41cmNWYUhOMGh6QjZ2NXQ3NE50NDB4VVhSTmFvbURjY2xCSU9sd3Q4ZjYySkEycC9qXG44M0VOZmRMclh2VXU5Tk1UaGtxWndaOWR6UndLN2wzVVpCcThOVFFVTzc0VzRNMnF4OG5yWHEzMWVXb2d4VVVJXG5GYzFYT1JoNWVjZWJlTDVtVWIyRTZVbG1EbU5nbTJmR2VTbW1pczh6aWVJK0tLWU9oaS9oWXR5ZWl4cmc3cnhQXG40djBWUnJFc3RjV0FldFJnWFdRWDBFbEF4czBWcnN5Ni92djNwRXRYaHg4d2Iyd2kyeFkxNGQ5SWg4SGRlTkkrXG4rM3dJYlp6NldWTTNmRDVRRkhWMkVaQkgrc29vMHBmS2oydEhzYUR6M0ZQTXVNeklMdDZVNlBUNEFMSWRBZ01CXG5BQUdqTWpBd01BOEdBMVVkRXdRSU1BWUJBZjhDQVFBd0hRWURWUjBPQkJZRUZKdXh6MFh3TjdQZGVNaHlKZmNmXG5tN3B5MUJLOU1BMEdDU3FHU0liM0RRRUJDd1VBQTRJQkFRQWhscGt6Njh4MmRHcE9MWDNGekFiOEVlK1kyT1YrXG5SV0Zwc01FOVpWRFUwNkpFVFBmUENqMDJQSDgybGdVbmM0amVSODFyUFNzSXQyc3NxbTJTNHpiMDJOaXA1OTVjXG5BcUNLdm1CZkVjOWhQUFcydWdwTnhUOFpSVTRMS3JxcFY0bko2bkJ2RHFtR3VINXVxOU5nOWw5U25NM2VLbWRaXG50SktjK1pOQVBLeFZBaXVlTFRkcjZXMlVibUtvWkFSUVEwSkxrRm5aT3huVWtyOHBRZnhVekVJVWtIZzJkV2FhXG5JLzR3bzRQbmk3eFhnZ0ZvUERwVnp0dS9pUDMzWEJMcVhKd3h4SFhocTluYzlKVS9rRVhEdDdqOEVnb3lKbzdKXG5qU0tjanBSZnBHa0U1Z3FxQjRTYTh3QXNBUFVLM2pScmV1eXRsbEF0UVVaUmJDdEhieGNsYzl5QVxuLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLSJdLCJ4NXQiOiJybnFhd2RoUXZ1bl9pa3JzcFBUaHpqblg2NEEifQ.eyJleHAiOjQ4ODUyNTg4NTUsImlzcyI6ImRpZDp4NTA5OjA6c2hhNTEyOkN0Z0FaZENpSEtsSmNIb1FOWkpod1dDSUN6ZS1EM2R1TzY1cDk1cWJfSDlxVTAtNVUzdXhESWpsR1p3S1ZYeXpBcEdRWWF1Q1oxUlFXZ2p6YWdMYWNROjpzYW46b3RoZXJOYW1lOjIuMTYuNTI4LjEuMTAwNy45OS4yMTEwLTEtODY0NDYtUy04NjQ0Ni0wMC4wMDAtODY0NDY6OnN1YmplY3Q6Tzpab3JnJTIwYmlqJTIwam91JTIwQi5WLjo6c3ViamVjdDpMOlV0cmVjaHQiLCJqdGkiOiJjNDkyNzU3My02MjI5LTQzMWMtODcyMC1iNGMxMmQ3MWNmMDMiLCJuYmYiOjE3MzE2NTg4NTUsInN1YiI6ImRpZDp3ZWI6em9yZ2JpampvdS50ZXN0LmludGVncmF0aW9uLnpvcmdiaWpqb3UuY29tOm51dHM6aWFtOjE5YjUxODlkLTIyZWUtNDIwZi05ZGRlLTQxNjhhNDE0ZmNhNSIsInZjIjp7IkBjb250ZXh0IjpbImh0dHBzOi8vd3d3LnczLm9yZy8yMDE4L2NyZWRlbnRpYWxzL3YxIl0sImNyZWRlbnRpYWxTdWJqZWN0IjpbeyJMIjoiVXRyZWNodCIsIk8iOiJab3JnIGJpaiBqb3UgQi5WLiIsImlkIjoiZGlkOndlYjp6b3JnYmlqam91LnRlc3QuaW50ZWdyYXRpb24uem9yZ2JpampvdS5jb206bnV0czppYW06MTliNTE4OWQtMjJlZS00MjBmLTlkZGUtNDE2OGE0MTRmY2E1Iiwib3RoZXJOYW1lIjoiMi4xNi41MjguMS4xMDA3Ljk5LjIxMTAtMS04NjQ0Ni1TLTg2NDQ2LTAwLjAwMC04NjQ0NiJ9XSwidHlwZSI6WyJWZXJpZmlhYmxlQ3JlZGVudGlhbCIsIlV6aVNlcnZlckNlcnRpZmljYXRlQ3JlZGVudGlhbCJdfX0.Vep71UX9iDOExVm6rwmjtVWPx7tYTkjSh7DJuBt20PT6qQa3l4jmyJERQsz7NA4_cSRoZV3HFoqt-wnExPXrD0OHZcTu0LesSkL9My7sOqaUDQjS1lCvceYIexMTfPhJMJm8qGTW0wg9SzJUWbanP6bmuRnq4KCfhCBJt14_HKPk3RfIMBPFhO5jRd_FPgXS3AoHJvcZget4mNDhVavn51Vk7m9HXju9PMKMTKx7l1VyX3MG04C_aTn5Jk9GDCZ9De5UcqyxXWO672MUUZ5_EBK6ajSW3xG2crcnRRHJvN0gBvKLBZJXTW3rOk9Ny0APkhCT4KrSGWc7cb0BKCp-vw")
		require.NoError(t, err)

		profile := &DutchNutsProfile{}
		identities, err := profile.identifiersFromCredential(*credential)

		require.NoError(t, err)
		assert.Len(t, identities, 1)
		assert.Equal(t, coolfhir.URANamingSystem, *identities[0].Identifier[0].System)
		assert.Equal(t, "86446", *identities[0].Identifier[0].Value)
		assert.Equal(t, "Zorg bij jou B.V.", *identities[0].Name)
		assert.Equal(t, "Utrecht", *identities[0].Address[0].City)
	})

}

func TestDutchNutsProfile_Identities(t *testing.T) {
	ctx := context.Background()
	identifier1 := fhir.Identifier{
		System: to.Ptr(coolfhir.URANamingSystem),
		Value:  to.Ptr("1234"),
	}
	identifier1VC := vc.VerifiableCredential{
		Type: []ssi.URI{ssi.MustParseURI("NutsUraCredential")},
		CredentialSubject: []interface{}{
			map[string]interface{}{
				"organization": map[string]interface{}{
					"ura": *identifier1.Value,
				},
			},
		},
	}
	identifier2 := fhir.Identifier{
		System: to.Ptr(coolfhir.URANamingSystem),
		Value:  to.Ptr("5678"),
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
		Type: []ssi.URI{ssi.MustParseURI("UziServerCertificateCredential")},
		CredentialSubject: []interface{}{
			map[string]interface{}{
				"otherName": "2.16.528.1.1007.99.2110-1-111-S-" + *identifier2.Value + "-00.000-222",
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
		assert.Contains(t, identities[0].Identifier, identifier1)
		assert.Contains(t, identities[1].Identifier, identifier2)
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
		assert.Contains(t, identities[0].Identifier, identifier1)
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
		assert.Contains(t, identities[0].Identifier, identifier1)
		assert.Contains(t, identities[1].Identifier, identifier2)

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
		assert.Contains(t, identities[0].Identifier, identifier1)
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
		assert.Contains(t, identities[0].Identifier, identifier1)
		assert.Contains(t, identities[1].Identifier, identifier2)

		// expire cache
		prof.identitiesRefreshedAt = prof.identitiesRefreshedAt.Add(-identitiesCacheTTL)

		identities, err = prof.Identities(ctx)
		require.NoError(t, err)
		require.Len(t, identities, 2)
	})
}
