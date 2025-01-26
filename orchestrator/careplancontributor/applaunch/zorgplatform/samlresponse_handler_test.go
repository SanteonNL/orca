package zorgplatform

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
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

func TestProfessionalSSOAssertion_Validate(t *testing.T) {
	sessionManager := user.NewSessionManager(time.Minute)
	s := &Service{
		sessionManager: sessionManager,
	}

	currentTime = func() time.Time {
		// Example assertion is valid from 2019-04-19T12:55:23.023Z to 2019-04-19T13:07:23.023Z
		return time.Date(2019, 4, 19, 13, 0, 0, 0, time.UTC)
	}
	ctx := context.Background()
	assertionXML, err := os.ReadFile("samlresponse_assertion_example.xml")
	require.NoError(t, err)
	assertionDocument := etree.NewDocument()
	require.NoError(t, assertionDocument.ReadFromBytes(assertionXML))

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

			ProfessionalSSOAssertion{
				Element:                    assertionDocument.Root(),
				ExpectedSigningCertificate: nil,
				ExpectedAudience:           "",
				ExpectedIssuer:             "",
			}
			assertionDocument.Root()

			// Validate Audience
			err := s.validateAudience(assertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}

			// Validate Issuer
			err = s.validateIssuer(assertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}

			// Extract Practitioner
			practitioner, err := s.extractPractitioner(ctx, assertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				subjectNameId := *practitioner.Identifier[0].Value + "@" + *practitioner.Identifier[0].System
				assert.Equal(t, tt.expectedSubj, subjectNameId)
			}

			// Extract Resource ID
			resourceID, err := s.extractResourceID(assertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedBSN, resourceID)
			}

			// Extract Workflow ID
			workflowID, err := s.extractWorkflowID(ctx, assertion)
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

func TestProfessionalSSOAssertion_validateSignature(t *testing.T) {
	t.Run("forged signature", func(t *testing.T) {
		sessionManager := user.NewSessionManager(time.Minute)

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

		forgedAssertion, err := s.createSAMLAssertion(launchContext, hcpTokenType)
		require.NoError(t, err)
		forgedSignedAssertion, err := s.signAssertion(forgedAssertion)
		require.NoError(t, err)

		err = (&ProfessionalSSOAssertion{Element: forgedSignedAssertion, ExpectedSigningCertificate: zorgplatformX509Cert}).validateSignature()
		require.EqualError(t, err, "unable to validate signature: Could not verify certificate against trusted certs")
	})

}
