package zorgplatform

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
)

// This test does not run the expiry checks as the Expires date in the assertion is in the past
func TestValidateAudienceIssuerAndExtractSubjectAndExtractResourceID(t *testing.T) {
	sessionManager := user.NewSessionManager()
	s := &Service{
		sessionManager: sessionManager,
	}

	assertionXML, err := os.ReadFile("assertion_example.xml")
	if err != nil {
		t.Fatalf("Failed to read assertion example XML: %v", err)
	}
	decryptedAssertion := etree.NewDocument()
	err = decryptedAssertion.ReadFromBytes(assertionXML)

	assert.NoError(t, err)

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
			expectedWorkflowId: " test123-workflow-id ",
			expectedError:      nil,
		},
		{
			name:               "Invalid audience",
			audience:           "invalid_audience",
			issuer:             "urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedSubj:       "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedBSN:        "999999205",
			expectedWorkflowId: " test123-workflow-id ",
			expectedError:      errors.New("invalid aud. Found [https://partner-application.nl] but expected [invalid_audience]"),
		},
		{
			name:               "Invalid issuer",
			audience:           "https://partner-application.nl",
			issuer:             "invalid_issuer",
			expectedSubj:       "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
			expectedBSN:        "999999205",
			expectedWorkflowId: " test123-workflow-id ",
			expectedError:      errors.New("invalid iss. Found [urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8] but expected [invalid_issuer]"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s.config.Audience = tt.audience
			s.config.Issuer = tt.issuer

			// Validate Audience
			err := s.validateAudience(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			// Validate Issuer
			err = s.validateIssuer(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			// Extract Subject
			subject, err := s.extractSubject(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSubj, subject)
			}

			// Extract Resource ID
			resourceID, err := s.extractResourceID(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBSN, resourceID)
			}

			// Extract Workflow ID
			workflowID, err := s.extractWorkflowID(decryptedAssertion)
			if tt.expectedError != nil && err != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
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
			created:       time.Now().Add(-5 * time.Minute).Format(time.RFC3339Nano),
			expires:       time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano),
			expectedError: nil,
		},
		{
			name:          "Token not yet valid",
			created:       time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano),
			expires:       time.Now().Add(10 * time.Minute).Format(time.RFC3339Nano),
			expectedError: errors.New("token is not valid at the current time"),
		},
		{
			name:          "Token expired",
			created:       time.Now().Add(-10 * time.Minute).Format(time.RFC3339Nano),
			expires:       time.Now().Add(-5 * time.Minute).Format(time.RFC3339Nano),
			expectedError: errors.New("token is not valid at the current time"),
		},
		{
			name:          "Invalid created time format",
			created:       "invalid_created_time",
			expires:       time.Now().Add(5 * time.Minute).Format(time.RFC3339Nano),
			expectedError: errors.New("invalid created time format: parsing time \"invalid_created_time\" as \"2006-01-02T15:04:05.999999999Z07:00\": cannot parse \"invalid_created_time\" as \"2006\""),
		},
		{
			name:          "Invalid expires time format",
			created:       time.Now().Add(-5 * time.Minute).Format(time.RFC3339Nano),
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

			err := s.validateTokenExpiry(doc)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
