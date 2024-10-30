package zorgplatform

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/braineet/saml/xmlenc"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"

	"github.com/beevik/etree"
	"github.com/rs/zerolog/log"
)

type LaunchContext struct {
	Bsn                string
	SubjectNameId      string
	Practitioner       fhir.Practitioner
	ServiceRequest     fhir.ServiceRequest
	WorkflowId         string
	DecryptedAssertion *etree.Element
}

// parseSamlResponse takes a SAML Response, validates it and extracts the SAML assertion, which is then returned as LaunchContext.
// If the SAML Assertion is encrypted, it decrypts it.
func (s *Service) parseSamlResponse(samlResponse string) (LaunchContext, error) {
	// TODO: Implement the SAML token validation logic
	doc := etree.NewDocument()
	decodedResponse, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to decode base64 SAML response: %w", err)
	}

	err = doc.ReadFromString(string(decodedResponse))

	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to parse XML: %w", err)
	}

	//TODO: Do we want to trim/cleanup values before validating?
	assertion, err := s.decryptAssertion(doc)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to decrypt assertion: %w", err)
	}

	// Validate the signature of the assertion using the public key of the Zorgplatform STS
	if err := s.validateSignature(assertion); err != nil {
		return LaunchContext{}, fmt.Errorf("invalid assertion signature: %w", err)
	}

	// TODO: Do we need this?
	//if err := s.validateAssertionExpiry(assertion); err != nil {
	//	return LaunchContext{}, fmt.Errorf("token has expired: %w", err)
	//}

	if err := s.validateAudience(assertion); err != nil {
		return LaunchContext{}, fmt.Errorf("invalid audience: %w", err)
	}
	if err := s.validateIssuer(assertion); err != nil {
		return LaunchContext{}, fmt.Errorf("invalid issuer: %w", err)
	}

	// TODO: Remove this before going to acceptance/production
	doc = etree.NewDocument()
	doc.SetRoot(assertion)
	xml, _ := doc.WriteToString()
	println("Decrypted assertion:", xml)

	// Extract Subject/NameID and log in the user
	practitioner, err := s.extractPractitioner(assertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract Practitioner from SAML Assertion.Subject: %w", err)
	}

	// Extract resource-id claim to select the correct patient
	resourceID, err := s.extractResourceID(assertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract resource-id: %w", err)
	}
	workflowID, err := s.extractWorkflowID(assertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract workflow-id: %w", err)
	}

	// // Process any other required attributes (claims)
	// if err := s.processAdditionalAttributes(assertion); err != nil {
	// 	return fmt.Errorf("unable to process additional attributes: %w", err)
	// }

	return LaunchContext{
		Bsn:                resourceID,
		Practitioner:       *practitioner,
		WorkflowId:         workflowID,
		DecryptedAssertion: assertion,
	}, nil

}

func (s *Service) decryptAssertion(doc *etree.Document) (*etree.Element, error) {
	el := doc.Root().FindElement("//EncryptedAssertion/xenc:EncryptedData")
	if el == nil {
		return nil, fmt.Errorf("EncryptedData element not found in the assertion")
	}
	decrypt, err := xmlenc.Decrypt(s.decryptCertificate, el)
	if err != nil {
		return nil, fmt.Errorf("unable to decrypt assertion: %w", err)
	}
	result := etree.NewDocument()
	if err := result.ReadFromBytes(decrypt); err != nil {
		return nil, fmt.Errorf("unable to parse decrypted assertion: %w", err)
	}
	return result.FindElement("Assertion"), nil
}

func (s *Service) validateAssertionExpiry(doc *etree.Document) error {
	createdElement := doc.Root().FindElement("//u:Timestamp/u:Created")
	expiresElement := doc.Root().FindElement("//u:Timestamp/u:Expires")

	if createdElement == nil || expiresElement == nil {
		return fmt.Errorf("timestamp elements not found in the assertion")
	}

	created := createdElement.Text()
	expires := expiresElement.Text()

	log.Trace().Msgf("Token created: %s", created)
	log.Trace().Msgf("Token expires: %s", expires)

	createdTime, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return fmt.Errorf("invalid created time format: %w", err)
	}

	expiresTime, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return fmt.Errorf("invalid expires time format: %w", err)
	}

	now := time.Now().UTC()
	if now.Before(createdTime) || now.After(expiresTime) {
		return fmt.Errorf("token is not valid at the current time")
	}

	return nil
}

// Validates the descrypted assertion signature using the public key of the Zorgplatform STS
func (s *Service) validateSignature(decryptedAssertion *etree.Element) error {
	signatureElement := decryptedAssertion.FindElement("Signature")
	if signatureElement == nil {
		return fmt.Errorf("signature element not found in the assertion")
	}
	//TODO: Implement when we can decrypt the assertions
	return nil
}

func (s *Service) validateAudience(decryptedAssertion *etree.Element) error {
	el := decryptedAssertion.FindElement("//AudienceRestriction/Audience")
	aud := el.Text()

	if aud == s.config.DecryptConfig.Audience {
		return nil
	}

	return fmt.Errorf("invalid aud. Found [%s] but expected [%s]", aud, s.config.DecryptConfig.Audience)
}

func (s *Service) validateIssuer(decryptedAssertion *etree.Element) error {

	el := decryptedAssertion.FindElement("//Issuer")
	iss := el.Text()

	if iss == s.config.DecryptConfig.Issuer {
		return nil
	}
	return fmt.Errorf("invalid iss. Found [%s] but expected [%s]", iss, s.config.DecryptConfig.Issuer)
}

func (s *Service) extractPractitioner(assertion *etree.Element) (*fhir.Practitioner, error) {
	var result fhir.Practitioner
	// Identifier (e.g.: USER1@2.16.840.1.113883.2.4.3.124.8.50.8)
	{
		el := assertion.FindElement("//Subject/NameID")
		if el == nil || strings.TrimSpace(el.Text()) == "" {
			return nil, errors.New("Subject.NameID not found")
		}
		parts := strings.Split(el.Text(), "@")
		identifier := fhir.Identifier{
			Value: to.Ptr(parts[0]),
		}
		if len(parts) > 1 {
			identifier.System = to.Ptr(parts[1])
		}
		result.Identifier = []fhir.Identifier{identifier}
	}
	// Name (e.g.: Jansen, Doctor - optional)
	{

		value, _ := getSubjectAttribute(assertion, "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name")
		if value != "" {
			parts := strings.Split(value, ",")
			result.Name = []fhir.HumanName{
				{
					Text: to.Ptr(value),
				},
			}
			result.Name[0].Family = to.Ptr(strings.TrimSpace(parts[0]))
			if len(parts) > 1 {
				result.Name[0].Given = []string{strings.TrimSpace(parts[1])}
			}
			if len(parts) > 2 {
				result.Name[0].Prefix = []string{strings.TrimSpace(parts[2])}
			}
		} else {
			log.Debug().Msg("Name attribute not found")
		}
	}
	// Role (e.g.: <Role code="223366009" codeSystem="2.16.840.1.113883.6.96" codeSystemName="SNOMED_CT"/>)
	{
		el := assertion.FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xacml:2.0:subject:role']/AttributeValue/Role")
		if el == nil {
			return nil, errors.New("subject Role not found")
		}
		const snomedCodeSystem = "2.16.840.1.113883.6.96"
		if el.SelectAttrValue("codeSystem", "") != snomedCodeSystem {
			// We could map this, but Zorgplatform probably only returns this code?
			return nil, errors.New("subject Role codeSystem is not " + snomedCodeSystem)
		}
		code := el.SelectAttrValue("code", "")
		if strings.TrimSpace(code) == "" {
			return nil, errors.New("subject Role code is empty")
		}
		result.Qualification = []fhir.PractitionerQualification{
			{
				Code: fhir.CodeableConcept{
					Coding: []fhir.Coding{
						{
							System:  to.Ptr("http://snomed.info/sct"),
							Code:    to.Ptr(code),
							Display: to.NilString(el.SelectAttrValue("displayName", "")),
						},
					},
				},
			},
		}
	}
	// E-mail (optional field)
	{
		value, _ := getSubjectAttribute(assertion, "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress")

		if value != "" {
			result.Telecom = append(result.Telecom, fhir.ContactPoint{
				System: to.Ptr(fhir.ContactPointSystemEmail),
				Value:  to.Ptr(value),
			})
		} else {
			log.Debug().Msg("Email attribute not found")
		}
	}

	return &result, nil
}

func (s *Service) extractResourceID(decryptedAssertion *etree.Element) (string, error) {
	// Find the Attribute element with the specified Name
	attributeElement := decryptedAssertion.FindElement("//AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xacml:1.0:resource:resource-id']/AttributeValue/InstanceIdentifier")
	if attributeElement == nil {
		return "", fmt.Errorf("resource-id attribute not found in the assertion")
	}

	// Extract the value of the extension attribute
	resourceID := attributeElement.SelectAttrValue("extension", "")
	if resourceID == "" {
		return "", fmt.Errorf("resource-id extension attribute not found")
	}

	return resourceID, nil
}

func (s *Service) extractWorkflowID(decryptedAssertion *etree.Element) (string, error) {
	workflowIdElement := decryptedAssertion.FindElement("//AttributeStatement/Attribute[@Name='http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id']/AttributeValue")
	if workflowIdElement == nil {
		return "", fmt.Errorf("workflow-id attribute not found in the assertion")
	}
	workflowId := workflowIdElement.Text()
	log.Debug().Msgf("Extracted workflow-id: %s", workflowId)

	return workflowId, nil
}

func (s *Service) processAdditionalAttributes(decryptedAssertion *etree.Document) error {
	// TODO: Implement the logic to process additional attributes (claims) from the assertion
	// This is a placeholder implementation
	return nil
}

func getSubjectAttribute(decryptedAssertion *etree.Element, name string) (string, error) {
	el := decryptedAssertion.FindElement("//AttributeStatement/Attribute[@Name='" + name + "']/AttributeValue")
	if el == nil || strings.TrimSpace(el.Text()) == "" {
		return "", fmt.Errorf("attribute not found: %s", name)
	}
	return strings.TrimSpace(el.Text()), nil
}
