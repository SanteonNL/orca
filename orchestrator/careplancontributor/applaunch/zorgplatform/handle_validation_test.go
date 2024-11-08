package zorgplatform

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// This test does not run the expiry checks as the Expires date in the assertion is in the past
func TestValidateAudienceIssuerAndExtractSubjectAndExtractResourceID(t *testing.T) {
	sessionManager := user.NewSessionManager()
	s := &Service{
		sessionManager: sessionManager,
	}

	assertionXML, err := os.ReadFile("assertion_example.xml")
	require.NoError(t, err)
	decryptedDocument := etree.NewDocument()
	err = decryptedDocument.ReadFromBytes(assertionXML)
	decryptedAssertion := decryptedDocument.FindElement("//Assertion")

	require.NoError(t, err)

	tests := []struct {
		name               string
		audience           string
		issuer             string
		expectedSubj       string
		expectedBSN        string
		expectedWorkflowId string
		expectedError      error
	}{
		{
			name:               "Happy flow",
			audience:           "https://partner-application.nl",
			issuer:             "urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedSubj:       "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedBSN:        "999999205",
			expectedWorkflowId: "test123-workflow-id",
			expectedError:      nil,
		},
		{
			name:               "Invalid audience",
			audience:           "invalid_audience",
			issuer:             "urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedSubj:       "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedBSN:        "999999205",
			expectedWorkflowId: "test123-workflow-id",
			expectedError:      errors.New("invalid aud. Found [https://partner-application.nl] but expected [invalid_audience]"),
		},
		{
			name:               "Invalid issuer",
			audience:           "https://partner-application.nl",
			issuer:             "invalid_issuer",
			expectedSubj:       "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedBSN:        "999999205",
			expectedWorkflowId: "test123-workflow-id",
			expectedError:      errors.New("invalid iss. Found [urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8] but expected [invalid_issuer]"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s.config.DecryptConfig.Audience = tt.audience
			s.config.DecryptConfig.Issuer = tt.issuer

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
			practitioner, err := s.extractPractitioner(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				subjectNameId := *practitioner.Identifier[0].Value + "@" + *practitioner.Identifier[0].System
				assert.Equal(t, tt.expectedSubj, subjectNameId)
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
			workflowID, err := s.extractWorkflowID(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedWorkflowId, workflowID)
			}

			//TODO: Add signature validation

		})
	}
}

func TestValidateTokenExpiry(t *testing.T) {
	sessionManager := user.NewSessionManager()
	s := &Service{
		sessionManager: sessionManager,
	}

	tests := []struct {
		name          string
		created       string
		expires       string
		expectedError error
	}{
		{
			name:          "Valid token",
			created:       FormatXSDDateTime(time.Now().Add(-5 * time.Minute)),
			expires:       FormatXSDDateTime(time.Now().Add(5 * time.Minute)),
			expectedError: nil,
		},
		{
			name:          "Token not yet valid",
			created:       FormatXSDDateTime(time.Now().Add(5 * time.Minute)),
			expires:       FormatXSDDateTime(time.Now().Add(10 * time.Minute)),
			expectedError: errors.New("token is not valid at the current time"),
		},
		{
			name:          "Token expired",
			created:       FormatXSDDateTime(time.Now().Add(-10 * time.Minute)),
			expires:       FormatXSDDateTime(time.Now().Add(-5 * time.Minute)),
			expectedError: errors.New("token is not valid at the current time"),
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
			doc := etree.NewDocument()
			root := doc.CreateElement("Assertion")
			timestamp := root.CreateElement("u:Timestamp")
			created := timestamp.CreateElement("u:Created")
			created.SetText(tt.created)
			expires := timestamp.CreateElement("u:Expires")
			expires.SetText(tt.expires)

			err := s.validateAssertionExpiry(doc)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestValidateZorgplatformForgedSignatureSelfSigned(t *testing.T) {
	sessionManager := user.NewSessionManager()

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
		SubjectNameId:  "Subject",
		WorkflowId:     "workflow-1234",
		ServiceRequest: fhir.ServiceRequest{},
	}

	s := &Service{
		sessionManager:        sessionManager,
		zorgplatformCert:      zorgplatformX509Cert,                 // used to verify the signature
		signingCertificateKey: keyPair.PrivateKey.(*rsa.PrivateKey), // used by the forger to sign the assertion
		signingCertificate:    keyPair.Certificate,                  // used by the forger to sign the assertion
	}

	forgedAssertion, err := s.createSAMLAssertion(launchContext)
	require.NoError(t, err)
	forgedSignedAssertion, err := s.signAssertion(forgedAssertion)
	require.NoError(t, err)

	err = s.validateZorgplatformSignature(forgedSignedAssertion)
	require.EqualError(t, err, "unable to validate signature: Could not verify certificate against trusted certs")
}
