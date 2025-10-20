package zorgplatform

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/lib/crypto"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// This test does not run the expiry checks as the Expires date in the assertion is in the past
func TestValidateAudienceIssuerAndExtractSubjectAndExtractResourceID(t *testing.T) {
	sessionManager := user.NewSessionManager[session.Data](time.Minute)
	s := &Service{
		sessionManager: sessionManager,
	}

	ctx := context.Background()
	assertionXML, err := os.ReadFile("assertion_example.xml")
	require.NoError(t, err)
	decryptedDocument := etree.NewDocument()
	err = decryptedDocument.ReadFromBytes(assertionXML)
	decryptedAssertion := decryptedDocument.FindElement("//Assertion")

	require.NoError(t, err)

	tests := []struct {
		name                   string
		audience               string
		issuer                 string
		expectedSubj           string
		expectedBSN            string
		expectedWorkflowId     string
		expectedRoleCode       string
		expectedRoleCodeSystem string
		expectedError          error
		currentTime            *time.Time
	}{
		{
			name:                   "Happy flow",
			audience:               "https://partner-application.nl",
			issuer:                 "urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedSubj:           "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedBSN:            "999999205",
			expectedWorkflowId:     "test123-workflow-id",
			expectedRoleCode:       "223366009",
			expectedRoleCodeSystem: "http://snomed.info/sct",
			expectedError:          nil,
		},
		{
			name:                   "Invalid audience",
			audience:               "invalid_audience",
			issuer:                 "urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedSubj:           "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedBSN:            "999999205",
			expectedWorkflowId:     "test123-workflow-id",
			expectedRoleCode:       "223366009",
			expectedRoleCodeSystem: "http://snomed.info/sct",
			expectedError:          errors.New("invalid aud. Found [https://partner-application.nl] but expected [invalid_audience]"),
		},
		{
			name:                   "Invalid issuer",
			audience:               "https://partner-application.nl",
			issuer:                 "invalid_issuer",
			expectedSubj:           "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedBSN:            "999999205",
			expectedWorkflowId:     "test123-workflow-id",
			expectedRoleCode:       "223366009",
			expectedRoleCodeSystem: "http://snomed.info/sct",
			expectedError:          errors.New("invalid iss. Found [urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8] but expected [invalid_issuer]"),
		},
		{
			name:                   "Happy flow",
			audience:               "https://partner-application.nl",
			issuer:                 "urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedSubj:           "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedBSN:            "999999205",
			expectedWorkflowId:     "test123-workflow-id",
			expectedRoleCode:       "223366009",
			expectedRoleCodeSystem: "http://snomed.info/sct",
			expectedError:          errors.New("current time 2025-01-01 01:01:00 +0000 UTC is not within the Conditions validity period [2019-04-19 12:55:23.023 +0000 UTC, 2019-04-19 13:07:23.023 +0000 UTC]"),
			currentTime:            to.Ptr(time.Date(2025, 1, 1, 1, 1, 0, 0, time.UTC)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s.config.DecryptConfig.Audience = tt.audience
			s.config.DecryptConfig.Issuer = tt.issuer

			if tt.currentTime == nil {
				now = func() time.Time {
					// Date of test fixture
					return time.Date(2019, 4, 19, 12, 57, 0, 0, time.UTC)
				}
			} else {
				now = func() time.Time {
					return *tt.currentTime
				}
			}
			defer func() {
				now = time.Now
			}()

			// Validate Audience
			err := s.validateAudience(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}

			// Validate Issuer
			err = s.validateIssuer(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}

			// Extract Practitioner
			practitioner, err := s.extractPractitioner(ctx, decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, HIX_LOCALUSER_SYSTEM, *practitioner.Identifier[0].System)
				assert.Equal(t, tt.expectedSubj, *practitioner.Identifier[0].Value)
			}

			// Extract Resource ID
			resourceID, err := s.extractResourceID(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedBSN, resourceID)
			}

			// Extract Workflow ID
			workflowID, err := s.extractWorkflowID(ctx, decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedWorkflowId, workflowID)
			}

			// Extract Practitioner Role
			practitionerRole, err := s.extractPractitionerRole(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedRoleCode, *practitionerRole.Code[0].Coding[0].Code)
				assert.Equal(t, tt.expectedRoleCodeSystem, *practitionerRole.Code[0].Coding[0].System)

				assert.Equal(t, []fhir.Identifier{
					{
						System: to.Ptr(HIX_LOCALUSER_SYSTEM),
						Value:  to.Ptr(tt.expectedSubj),
					},
				}, practitionerRole.Identifier)
			}

			//TODO: Add signature validation

		})
	}
}

func TestValidateTokenExpiry(t *testing.T) {
	sessionManager := user.NewSessionManager[session.Data](time.Minute)
	s := &Service{
		sessionManager: sessionManager,
	}

	tests := []struct {
		name          string
		created       string
		expires       string
		expectedError error
		currentTime   *time.Time
	}{
		{
			name:          "Valid token",
			created:       FormatXSDDateTime(time.Now().Add(-5 * time.Minute)),
			expires:       FormatXSDDateTime(time.Now().Add(5 * time.Minute)),
			expectedError: nil,
		},
		{
			name:          "Token not yet valid",
			created:       "2024-01-01T00:00:00.000Z",
			expires:       "2024-01-01T01:00:00.000Z",
			expectedError: errors.New("SecurityTokenResponse is not valid at the current time: 2025-01-01 01:01:00 +0000 UTC, expected between [2024-01-01 00:00:00 +0000 UTC, 2024-01-01 01:00:00 +0000 UTC]"),
			currentTime:   to.Ptr(time.Date(2025, 1, 1, 1, 1, 0, 0, time.UTC)), // Simulate a future time
		},
		{
			name:          "Token expired",
			created:       "2024-01-01T00:00:00.000Z",
			expires:       "2024-01-01T00:30:00.000Z",
			expectedError: errors.New("SecurityTokenResponse is not valid at the current time: 2024-01-01 01:00:00 +0000 UTC, expected between [2024-01-01 00:00:00 +0000 UTC, 2024-01-01 00:30:00 +0000 UTC]"),
			currentTime:   to.Ptr(time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)), // Simulate a time after the token has expired
		},
		{
			name:          "Invalid created time format",
			created:       "invalid_created_time",
			expires:       FormatXSDDateTime(time.Now().Add(5 * time.Minute)),
			expectedError: errors.New("invalid created time format: parsing time \"invalid_created_time\" as \"2006-01-02T15:04:05.999999999Z07:00\": cannot parse \"invalid_created_time\" as \"2006\""),
		},
		{
			name:          "Invalid expires time format",
			created:       FormatXSDDateTime(time.Now().Add(-5 * time.Minute)),
			expires:       "invalid_expires_time",
			expectedError: errors.New("invalid expires time format: parsing time \"invalid_expires_time\" as \"2006-01-02T15:04:05.999999999Z07:00\": cannot parse \"invalid_expires_time\" as \"2006\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.currentTime != nil {
				now = func() time.Time {
					return *tt.currentTime
				}
			}
			defer func() {
				now = time.Now
			}()

			doc := etree.NewDocument()
			root := doc.CreateElement("trust:RequestSecurityTokenResponseCollection")
			response := root.CreateElement("trust:RequestSecurityTokenResponse")
			lifetime := response.CreateElement("trust:Lifetime")
			created := lifetime.CreateElement("u:Created")
			created.SetText(tt.created)
			expires := lifetime.CreateElement("u:Expires")
			expires.SetText(tt.expires)

			err := s.validateResponseExpiry(root)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestValidateZorgplatformForgedSignatureSelfSigned(t *testing.T) {
	sessionManager := user.NewSessionManager[session.Data](time.Minute)

	zorgplatformCertData, err := os.ReadFile("zorgplatform.online.pem")
	require.NoError(t, err)
	zorgplatformCertBlock, _ := pem.Decode(zorgplatformCertData)
	require.NotNil(t, zorgplatformCertBlock)
	zorgplatformX509Cert, err := x509.ParseCertificate(zorgplatformCertBlock.Bytes)
	require.NoError(t, err)

	keyPair, err := tls.LoadX509KeyPair("test-certificate.pem", "test-key.pem")
	require.NoError(t, err)

	launchContext := &LaunchContext{
		Bsn: "123456789",
		Practitioner: fhir.Practitioner{
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("urn:oid:2.16.840.1.113883.4.1"),
					Value:  to.Ptr("999999999"),
				},
			},
		},
		WorkflowId:     "workflow-1234",
		ServiceRequest: fhir.ServiceRequest{},
	}

	s := &Service{
		sessionManager:        sessionManager,
		zorgplatformSignCerts: []*x509.Certificate{zorgplatformX509Cert}, // used to verify the signature
		signingCertificateKey: keyPair.PrivateKey.(*rsa.PrivateKey),      // used by the forger to sign the assertion
		signingCertificate:    keyPair.Certificate,                       // used by the forger to sign the assertion
	}

	forgedAssertion, err := s.createSAMLAssertion(launchContext, hcpTokenType)
	require.NoError(t, err)
	forgedSignedAssertion, err := s.signAssertion(forgedAssertion)
	require.NoError(t, err)

	err = s.validateZorgplatformSignature(forgedSignedAssertion)
	require.EqualError(t, err, "unable to validate signature: Could not verify certificate against trusted certs")
}

func TestService_parseSamlResponse(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		certificate, err := tls.LoadX509KeyPair("test-certificate.pem", "test-key.pem")
		require.NoError(t, err)
		samlResponse := createSAMLResponse(t, certificate.Leaf)
		now = func() time.Time {
			return time.Date(2024, 11, 06, 15, 57, 0, 0, time.UTC)
		}
		defer func() {
			now = time.Now
		}()
		s := &Service{
			decryptCertificate: crypto.RsaSuite{
				PrivateKey: certificate.PrivateKey.(*rsa.PrivateKey),
				Cert:       certificate.Leaf,
			},
			zorgplatformSignCerts: []*x509.Certificate{certificate.Leaf},
			config: Config{
				DecryptConfig: DecryptConfig{
					Audience: "https://partner-application.nl",
					Issuer:   "unit-test",
				},
			},
		}
		ctx := context.Background()

		actual, err := s.parseSamlResponse(ctx, samlResponse)

		require.NoError(t, err)
		assert.NotEmpty(t, actual)
		assert.Equal(t, "999999151", actual.Bsn)
		assert.Len(t, actual.Practitioner.Identifier, 1)
		assert.Equal(t, HIX_LOCALUSER_SYSTEM, *actual.Practitioner.Identifier[0].System)
		assert.Equal(t, "999999999@urn:oid:2.16.840.1.113883.4.1", *actual.Practitioner.Identifier[0].Value)
		assert.Equal(t, "b526e773-e1a6-4533-bd00-1360c97e745f", actual.WorkflowId)

	})
	t.Run("<Error> response", func(t *testing.T) {
		s := &Service{}
		doc := etree.NewDocument()
		root := doc.CreateElement("Error")
		root.CreateAttr("LogId", "0002297112")
		ctx := context.Background()
		xmlStr, _ := doc.WriteToString()
		xmlBase64Encoded := base64.StdEncoding.EncodeToString([]byte(xmlStr))

		actual, err := s.parseSamlResponse(ctx, xmlBase64Encoded)

		assert.Empty(t, actual)
		require.EqualError(t, err, "received SAMLResponse contains an error tag and cannot be processed, check error log for details")
	})
}

func TestService_parseAssertion(t *testing.T) {
	assertionXML, err := os.ReadFile("saml_hix_sso_assertion.xml")
	require.NoError(t, err)
	doc := etree.NewDocument()
	err = doc.ReadFromBytes(assertionXML)
	require.NoError(t, err)

	launchContext, err := (&Service{}).parseAssertion(context.Background(), doc.Root())

	require.NoError(t, err)
	assert.Equal(t, "999999102", launchContext.Bsn)
	assert.Len(t, launchContext.Practitioner.Identifier, 1)
	assert.Equal(t, HIX_LOCALUSER_SYSTEM, *launchContext.Practitioner.Identifier[0].System)
	assert.Equal(t, "1234@1.2.3.4.5", *launchContext.Practitioner.Identifier[0].Value)
	assert.Equal(t, "Arts", *launchContext.Practitioner.Name[0].Family)
	assert.Equal(t, "H.", launchContext.Practitioner.Name[0].Given[0])
	assert.Equal(t, "f8cab0af-901b-417e-8bb4-5198e2c47732", launchContext.WorkflowId)
}
