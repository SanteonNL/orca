package zorgplatform

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/braineet/saml/xmlenc"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"

	"github.com/beevik/etree"
	"github.com/rs/zerolog/log"
)

// currentTime is a variable that holds a function that returns the current time.
// This variable is used to mock the current time in tests.
var currentTime = time.Now

type LaunchContext struct {
	Bsn            string
	SubjectNameId  string
	Practitioner   fhir.Practitioner
	ServiceRequest fhir.ServiceRequest
	WorkflowId     string
}

// parseSAMLResponse takes a SAML Response, validates it and extracts the SAML assertion, which is then returned as LaunchContext.
// If the SAML Assertion is encrypted, it decrypts it.
func (s *Service) parseSAMLResponse(ctx context.Context, samlResponse string) (*LaunchContext, error) {
	// TODO: Implement the SAML token validation logic
	doc := etree.NewDocument()
	decodedResponse, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil {
		return nil, fmt.Errorf("unable to decode base64 SAML response: %w", err)
	}

	err = doc.ReadFromString(string(decodedResponse))

	if err != nil {
		return nil, fmt.Errorf("unable to parse XML: %w", err)
	}

	//wsSecurity := doc.Root().FindElement("./s:Header/o:Security")
	//if wsSecurity == nil {
	//	return nil, errors.New("ws-security header not found")
	//}

	//TODO: Do we want to trim/cleanup values before validating?
	assertion, err := s.decryptAssertion(doc)
	if err != nil {
		return nil, fmt.Errorf("unable to decrypt assertion: %w", err)
	}

	// TODO: Remove this before going to acceptance/production
	doc = etree.NewDocument()
	doc.SetRoot(assertion)
	xml, _ := doc.WriteToString()
	println("Decrypted assertion:", xml)

	return (&ProfessionalSSOAssertion{
		Element:                    assertion,
		ExpectedSigningCertificate: s.zorgplatformCert,
	}).Validate(ctx)
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

type ProfessionalSSOAssertion struct {
	*etree.Element
	ExpectedSigningCertificate *x509.Certificate
	ExpectedAudience           string
	ExpectedIssuer             string
}

func (a *ProfessionalSSOAssertion) Validate(ctx context.Context) (*LaunchContext, error) {
	if err := a.validateSignature(); err != nil {
		return nil, fmt.Errorf("invalid assertion signature: %w", err)
	}
	if err := a.validateAudience(); err != nil {
		return nil, fmt.Errorf("invalid audience: %w", err)
	}
	if err := a.validateIssuer(); err != nil {
		return nil, fmt.Errorf("invalid issuer: %w", err)
	}

	// Extract Subject/NameID and log in the user
	practitioner, err := a.extractPractitioner(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to extract Practitioner from SAML Assertion.Subject: %w", err)
	}

	// Extract resource-id claim to select the correct patient
	resourceID, err := a.extractResourceID()
	if err != nil {
		return nil, fmt.Errorf("unable to extract resource-id: %w", err)
	}
	workflowID, err := a.extractWorkflowID(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to extract workflow-id: %w", err)
	}

	return &LaunchContext{
		Bsn:          resourceID,
		Practitioner: *practitioner,
		WorkflowId:   workflowID,
	}, nil
}

// validateSignature checks the decrypted assertion signature using the public key of the Zorgplatform STS
func (a *ProfessionalSSOAssertion) validateSignature() error {
	signatureElement := a.FindElement("Signature")
	if signatureElement == nil {
		return fmt.Errorf("signature element not found in the assertion")
	}

	validationContext := dsig.NewDefaultValidationContext(&dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{a.ExpectedSigningCertificate},
	})

	_, err := validationContext.Validate(a.Element)
	if err != nil {
		return fmt.Errorf("unable to validate signature: %w", err)
	}
	return nil //valid
}

func (a *ProfessionalSSOAssertion) validateAudience() error {
	conditionsEl := a.FindElement("Conditions")
	if conditionsEl == nil {
		return fmt.Errorf("conditions element not found in the assertion")
	}
	if notBeforeEl, notAfterEl := conditionsEl.SelectAttr("NotBefore"), conditionsEl.SelectAttr("NotOnOrAfter"); notBeforeEl != nil && notAfterEl != nil {
		if isValid, err := xmlTimeBetweenInclusive(notBeforeEl.Value, notAfterEl.Value, currentTime()); err != nil {
			return fmt.Errorf("unable to validate audience timestamps: %w", err)
		} else if !isValid {
			return fmt.Errorf("audience token is expired")
		}
	}

	audienceEl := conditionsEl.FindElement("AudienceRestriction/Audience")
	aud := strings.TrimSpace(audienceEl.Text())
	if aud != a.ExpectedAudience {
		return fmt.Errorf("invalid aud. Found [%s] but expected [%s]", aud, a.ExpectedAudience)
	}
	return nil
}

func (a *ProfessionalSSOAssertion) validateIssuer() error {
	el := a.FindElement("//Issuer")
	iss := el.Text()

	if iss == a.ExpectedIssuer {
		return nil
	}
	return fmt.Errorf("invalid iss. Found [%s] but expected [%s]", iss, a.ExpectedIssuer)
}

func (a *ProfessionalSSOAssertion) extractPractitioner(ctx context.Context) (*fhir.Practitioner, error) {
	var result fhir.Practitioner
	// Identifier (e.g.: USER1@2.16.840.1.113883.2.4.3.124.8.50.8)
	{
		el := a.FindElement("//Subject/NameID")
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

		value, _ := getSubjectAttribute(a.Element, "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name")
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
			log.Debug().Ctx(ctx).Msg("Name attribute not found")
		}
	}
	// Role (e.g.: <Role code="223366009" codeSystem="2.16.840.1.113883.6.96" codeSystemName="SNOMED_CT"/>)
	{
		el := a.FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xacml:2.0:subject:role']/AttributeValue/Role")
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
		value, _ := getSubjectAttribute(a.Element, "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress")

		if value != "" {
			result.Telecom = append(result.Telecom, fhir.ContactPoint{
				System: to.Ptr(fhir.ContactPointSystemEmail),
				Value:  to.Ptr(value),
			})
		} else {
			log.Debug().Ctx(ctx).Msg("Email attribute not found")
		}
	}

	return &result, nil
}

func (a *ProfessionalSSOAssertion) extractResourceID() (string, error) {
	// Find the Attribute element with the specified Name
	attributeElement := a.FindElement("//AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xacml:1.0:resource:resource-id']/AttributeValue/InstanceIdentifier")
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

func (a *ProfessionalSSOAssertion) extractWorkflowID(ctx context.Context) (string, error) {
	workflowIdElement := a.FindElement("//AttributeStatement/Attribute[@Name='http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id']/AttributeValue")
	if workflowIdElement == nil {
		return "", fmt.Errorf("workflow-id attribute not found in the assertion")
	}
	workflowId := workflowIdElement.Text()
	log.Debug().Ctx(ctx).Msgf("Extracted workflow-id: %s", workflowId)

	return workflowId, nil
}

func getSubjectAttribute(decryptedAssertion *etree.Element, name string) (string, error) {
	el := decryptedAssertion.FindElement("//AttributeStatement/Attribute[@Name='" + name + "']/AttributeValue")
	if el == nil || strings.TrimSpace(el.Text()) == "" {
		return "", fmt.Errorf("attribute not found: %s", name)
	}
	return strings.TrimSpace(el.Text()), nil
}
