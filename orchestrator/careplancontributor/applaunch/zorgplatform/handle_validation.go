package zorgplatform

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/braineet/saml/xmlenc"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"

	"github.com/beevik/etree"
	"github.com/rs/zerolog/log"
)

var now = func() time.Time {
	return time.Now()
}

type LaunchContext struct {
	Bsn              string
	SubjectNameId    string
	Practitioner     fhir.Practitioner
	PractitionerRole fhir.PractitionerRole
	ServiceRequest   fhir.ServiceRequest
	WorkflowId       string
}

const HIX_LOCALUSER_SYSTEM = "https://www.cwz.nl/hix-user"
const HIX_ORG_OID_SYSTEM = "https://www.cwz.nl/hix-org-oid"

// parseSamlResponse takes a SAML Response, validates it and extracts the SAML assertion, which is then returned as LaunchContext.
// If the SAML Assertion is encrypted, it decrypts it.
func (s *Service) parseSamlResponse(ctx context.Context, samlResponse string) (LaunchContext, error) {
	doc := etree.NewDocument()
	decodedResponse, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to decode base64 SAML response: %w", err)
	}

	if log.Ctx(ctx).Enabled(zerolog.DebugLevel) {
		log.Ctx(ctx).Debug().Msgf("Zorgplatform SAMLResponse: %s", samlResponse)
	}

	err = doc.ReadFromString(string(decodedResponse))

	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to parse XML: %w", err)
	}

	if doc.Root().Tag == "Error" {
		log.Ctx(ctx).Error().Msgf("error tag as SAMLResponse: %s", decodedResponse)
		return LaunchContext{}, errors.New("SAMLResponse from server contains an error, see log for details")
	}

	if err := s.validateResponseExpiry(doc.Root()); err != nil {
		return LaunchContext{}, fmt.Errorf("SAML response expiration: %w", err)
	}

	assertion, err := s.decryptAssertion(doc)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to decrypt assertion: %w", err)
	}

	if log.Ctx(ctx).GetLevel() >= zerolog.DebugLevel {
		debugDoc := etree.NewDocument()
		debugDoc.SetRoot(assertion)
		xml, _ := debugDoc.WriteToString()
		log.Ctx(ctx).Debug().Msgf("Zorgplatform SAMLResponse assertion: %s", xml)
	}

	// Validate the signature of the assertion using the public key of the Zorgplatform STS
	if err := s.validateZorgplatformSignature(assertion); err != nil {
		return LaunchContext{}, fmt.Errorf("invalid assertion signature: %w", err)
	}

	if err := s.validateAudience(assertion); err != nil {
		return LaunchContext{}, fmt.Errorf("invalid audience: %w", err)
	}
	if err := s.validateIssuer(assertion); err != nil {
		return LaunchContext{}, fmt.Errorf("invalid issuer: %w", err)
	}

	// Extract Subject/NameID and log in the user
	practitioner, err := s.extractPractitioner(ctx, assertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract Practitioner from SAML Assertion.Subject: %w", err)
	}

	practitionerRole, err := s.extractPractitionerRole(assertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract PractitionerRole from SAML Assertion.Subject: %w", err)
	}

	// Extract resource-id claim to select the correct patient
	resourceID, err := s.extractResourceID(assertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract resource-id: %w", err)
	}
	workflowID, err := s.extractWorkflowID(ctx, assertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract workflow-id: %w", err)
	}

	// // Process any other required attributes (claims)
	// if err := s.processAdditionalAttributes(assertion); err != nil {
	// 	return fmt.Errorf("unable to process additional attributes: %w", err)
	// }

	return LaunchContext{
		Bsn:              resourceID,
		Practitioner:     *practitioner,
		PractitionerRole: *practitionerRole,
		WorkflowId:       workflowID,
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

func (s *Service) validateResponseExpiry(rstResponseCollection *etree.Element) error {
	securityTokenResponses := rstResponseCollection.FindElements(rstResponseCollection.Space + ":RequestSecurityTokenResponse")
	if len(securityTokenResponses) != 1 {
		return fmt.Errorf("expected 1 RequestSecurityTokenResponse, found %d", len(securityTokenResponses))
	}
	securityTokenResponse := securityTokenResponses[0]
	createdElement := securityTokenResponse.FindElement(rstResponseCollection.Space + ":Lifetime/Created")
	expiresElement := securityTokenResponse.FindElement(rstResponseCollection.Space + ":Lifetime/Expires")

	if createdElement == nil || expiresElement == nil {
		return fmt.Errorf("timestamp elements not found in the assertion")
	}

	created := strings.TrimSpace(createdElement.Text())
	expires := strings.TrimSpace(expiresElement.Text())

	createdTime, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return fmt.Errorf("invalid created time format: %w", err)
	}

	expiresTime, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return fmt.Errorf("invalid expires time format: %w", err)
	}

	currentTime := now()
	if currentTime.Before(createdTime) || currentTime.After(expiresTime) {
		return fmt.Errorf("SecurityTokenResponse is not valid at the current time: %s, expected between [%s, %s]", currentTime, createdTime, expiresTime)
	}

	return nil
}

// Validates the descrypted assertion signature using the public key of the Zorgplatform STS
func (s *Service) validateZorgplatformSignature(decryptedAssertion *etree.Element) error {
	signatureElement := decryptedAssertion.FindElement("Signature")
	if signatureElement == nil {
		return fmt.Errorf("signature element not found in the assertion")
	}

	validationContext := dsig.NewDefaultValidationContext(&dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{s.zorgplatformCert},
	})

	_, err := validationContext.Validate(decryptedAssertion)
	if err != nil {
		return fmt.Errorf("unable to validate signature: %w", err)
	}
	return nil //valid
}

func (s *Service) validateAudience(assertion *etree.Element) error {
	// Check Conditions.NotBefore and NotOnOrAfter
	conditionsElements := assertion.FindElements("Conditions")
	if len(conditionsElements) != 1 {
		return fmt.Errorf("expected exactly one Conditions element, found %d", len(conditionsElements))
	}
	conditionsElement := conditionsElements[0]
	notBefore, err := time.Parse(time.RFC3339Nano, conditionsElement.SelectAttrValue("NotBefore", ""))
	if err != nil {
		return fmt.Errorf("invalid Conditions.NotBefore: %w", err)
	}
	notOnOrAfter, err := time.Parse(time.RFC3339Nano, conditionsElement.SelectAttrValue("NotOnOrAfter", ""))
	if err != nil {
		return fmt.Errorf("invalid Conditions.NotOnOrAfter: %w", err)
	}
	currentTime := now()
	if currentTime.Before(notBefore) || currentTime.After(notOnOrAfter) {
		return fmt.Errorf("current time %s is not within the Conditions validity period [%s, %s]", currentTime, notBefore, notOnOrAfter)
	}

	// Check AudienceRestriction
	el := assertion.FindElement("//AudienceRestriction/Audience")
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

func (s *Service) extractPractitionerRole(assertion *etree.Element) (*fhir.PractitionerRole, error) {
	var result fhir.PractitionerRole

	{
		// Role (e.g.: <Role code="223366009" codeSystem="2.16.840.1.113883.6.96" codeSystemName="SNOMED_CT"/>)
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
		result.Code = []fhir.CodeableConcept{
			{
				Coding: []fhir.Coding{
					{
						System:  to.Ptr("http://snomed.info/sct"),
						Code:    to.Ptr(code),
						Display: to.NilString(el.SelectAttrValue("displayName", "")),
					},
				},
			},
		}
	}
	// Identifier (e.g.: USER1@2.16.840.1.113883.2.4.3.124.8.50.8)
	{
		el := assertion.FindElement("//Subject/NameID")
		if el == nil || strings.TrimSpace(el.Text()) == "" {
			return nil, errors.New("Subject.NameID not found")
		}
		parts := strings.Split(el.Text(), "@")

		if len(parts) != 2 {
			return nil, errors.New("Subject.NameID is not in the correct format - Expecting 2 parts on splitting by '@'")
		}

		userIdentifier := fhir.Identifier{
			System: to.Ptr(HIX_LOCALUSER_SYSTEM),
			Value:  to.Ptr(parts[0]),
		}

		orgIdentifier := fhir.Identifier{
			System: to.Ptr(HIX_ORG_OID_SYSTEM),
			Value:  to.Ptr(parts[1]),
		}

		result.Identifier = []fhir.Identifier{userIdentifier, orgIdentifier}
	}

	return &result, nil
}

func (s *Service) extractPractitioner(ctx context.Context, assertion *etree.Element) (*fhir.Practitioner, error) {
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
			log.Ctx(ctx).Debug().Msg("Name attribute not found")
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
			log.Ctx(ctx).Debug().Msg("Email attribute not found")
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

func (s *Service) extractWorkflowID(ctx context.Context, decryptedAssertion *etree.Element) (string, error) {
	workflowIdElement := decryptedAssertion.FindElement("//AttributeStatement/Attribute[@Name='http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id']/AttributeValue")
	if workflowIdElement == nil {
		return "", fmt.Errorf("workflow-id attribute not found in the assertion")
	}
	workflowId := workflowIdElement.Text()
	log.Ctx(ctx).Debug().Msgf("Extracted workflow-id: %s", workflowId)

	return workflowId, nil
}

func getSubjectAttribute(decryptedAssertion *etree.Element, name string) (string, error) {
	el := decryptedAssertion.FindElement("//AttributeStatement/Attribute[@Name='" + name + "']/AttributeValue")
	if el == nil || strings.TrimSpace(el.Text()) == "" {
		return "", fmt.Errorf("attribute not found: %s", name)
	}
	return strings.TrimSpace(el.Text()), nil
}
