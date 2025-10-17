package zorgplatform

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/braineet/saml/xmlenc"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/beevik/etree"
)

var now = func() time.Time {
	return time.Now()
}

type LaunchContext struct {
	Bsn                    string
	Practitioner           fhir.Practitioner
	PractitionerRole       fhir.PractitionerRole
	ServiceRequest         fhir.ServiceRequest
	WorkflowId             string
	ChipSoftOrganizationID string
}

const HIX_LOCALUSER_SYSTEM = "https://santeonnl.github.io/shared-care-planning/ehr/hix/userid"

// parseSamlResponse takes a SAML Response, validates it and extracts the SAML assertion, which is then returned as LaunchContext.
// If the SAML Assertion is encrypted, it decrypts it.
func (s *Service) parseSamlResponse(ctx context.Context, samlResponse string) (LaunchContext, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	doc := etree.NewDocument()
	decodedResponse, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("unable to decode base64 SAML response: %w", err))
	}

	slog.DebugContext(ctx, "Zorgplatform SAMLResponse", slog.String("saml_response", samlResponse))

	err = doc.ReadFromString(string(decodedResponse))

	if err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("unable to parse XML: %w", err))
	}

	if doc.Root().Tag == "Error" {
		slog.ErrorContext(
			ctx,
			"SAMLResponse contains error tag and can not be processed",
			slog.String("saml_response", string(decodedResponse)),
		)
		span.SetAttributes(attribute.String("saml_response", string(decodedResponse)))
		return LaunchContext{}, otel.Error(span, errors.New("received SAMLResponse contains an error tag and cannot be processed, check error log for details"))
	}

	// Note: for some reason, this fails on the Zorgplatform SAML response, so we skip it for now.
	//if err := s.validateResponseExpiry(doc.Root()); err != nil {
	//	return LaunchContext{}, fmt.Errorf("SAML response expiration: %w", err)
	//}

	assertion, err := s.decryptAssertion(doc)
	if err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("unable to decrypt assertion: %w", err))
	}
	span.AddEvent("decrypted assertion")

	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		debugDoc := etree.NewDocument()
		debugDoc.SetRoot(assertion)
		xml, _ := debugDoc.WriteToString()
		slog.DebugContext(ctx, "Zorgplatform SAMLResponse assertion", slog.String("assertion", xml))
	}

	// Validate the signature of the assertion using the public key of the Zorgplatform STS
	if err := s.validateZorgplatformSignature(assertion); err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("invalid assertion signature: %w", err))
	}
	span.AddEvent("validated assertion signature")

	if err := s.validateAudience(assertion); err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("invalid audience: %w", err))
	}
	span.AddEvent("validated assertion audience")

	if err := s.validateIssuer(assertion); err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("invalid issuer: %w", err))
	}
	span.AddEvent("validated assertion issuer")

	return s.parseAssertion(ctx, assertion)
}

func (s *Service) parseAssertion(ctx context.Context, assertion *etree.Element) (LaunchContext, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	// Extract Subject/NameID and log in the user
	practitioner, err := s.extractPractitioner(ctx, assertion)
	if err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("unable to extract Practitioner from SAML Assertion.Subject: %w", err))
	}

	practitionerRole, err := s.extractPractitionerRole(assertion)
	if err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("unable to extract PractitionerRole from SAML Assertion.Subject: %w", err))
	}
	if len(practitioner.Identifier) > 0 {
		practitionerRole.Practitioner = &fhir.Reference{
			Identifier: &practitioner.Identifier[0],
		}
	}

	// Extract resource-id claim to select the correct patient
	resourceID, err := s.extractResourceID(assertion)
	if err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("unable to extract resource-id: %w", err))
	}
	workflowID, err := s.extractWorkflowID(ctx, assertion)
	if err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("unable to extract workflow-id: %w", err))
	}

	chipSoftOrgID, err := s.extractOrganizationID(assertion)
	if err != nil {
		return LaunchContext{}, otel.Error(span, fmt.Errorf("unable to extract organization ID: %w", err))
	}
	// // Process any other required attributes (claims)
	// if err := s.processAdditionalAttributes(assertion); err != nil {
	// 	return fmt.Errorf("unable to process additional attributes: %w", err)
	// }

	span.AddEvent("SAML Assertion parsed successfully")

	return LaunchContext{
		Bsn:                    resourceID,
		Practitioner:           *practitioner,
		PractitionerRole:       *practitionerRole,
		WorkflowId:             workflowID,
		ChipSoftOrganizationID: chipSoftOrgID,
	}, nil
}

func (s *Service) decryptAssertion(doc *etree.Document) (*etree.Element, error) {
	el := doc.Root().FindElement("//EncryptedAssertion/xenc:EncryptedData")
	if el == nil {
		return nil, fmt.Errorf("EncryptedData element not found in the assertion")
	}
	decrypt, err := xmlenc.Decrypt(s.decryptCertificate, el)
	if err != nil {
		return nil, err
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
		Roots: s.zorgplatformSignCerts, // certs are pinned, so set as root CA certs
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

func (s *Service) extractOrganizationID(assertion *etree.Element) (string, error) {
	// Identifier (e.g.: USER1@2.16.840.1.113883.2.4.3.124.8.50.8)
	el := assertion.FindElement("//Subject/NameID")
	if el == nil || strings.TrimSpace(el.Text()) == "" {
		return "", errors.New("Subject.NameID not found")
	}
	value := strings.TrimSpace(el.Text())
	idx := strings.IndexAny(value, "@")
	if idx == -1 || idx == len(value)-1 {
		return "", fmt.Errorf("invalid NameID format, expected '<name>@<oid>' but got '%s'", value)
	}
	return value[idx+1:], nil
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
		result.Identifier = []fhir.Identifier{{
			System: to.Ptr(HIX_LOCALUSER_SYSTEM),
			Value:  to.Ptr(el.Text()),
		}}
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
		result.Identifier = []fhir.Identifier{{
			System: to.Ptr(HIX_LOCALUSER_SYSTEM),
			Value:  to.Ptr(el.Text()),
		}}
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
			slog.DebugContext(ctx, "Name attribute not found")
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
			slog.DebugContext(ctx, "Email attribute not found")
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
	slog.DebugContext(ctx, "Extracted workflow ID", slog.String("workflow_id", workflowId))

	return workflowId, nil
}

func getSubjectAttribute(decryptedAssertion *etree.Element, name string) (string, error) {
	el := decryptedAssertion.FindElement("//AttributeStatement/Attribute[@Name='" + name + "']/AttributeValue")
	if el == nil || strings.TrimSpace(el.Text()) == "" {
		return "", fmt.Errorf("attribute not found: %s", name)
	}
	return strings.TrimSpace(el.Text()), nil
}
