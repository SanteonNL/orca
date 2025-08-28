package zorgplatform

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/beevik/etree"
	"github.com/google/uuid"
	dsig "github.com/russellhaering/goxmldsig"
)

type SecureTokenService interface {
	RequestAccessToken(ctx context.Context, launchContext LaunchContext, tokenType TokenType) (string, error)
}

type TokenType struct {
	Subject      func(element *etree.Element, launchContext *LaunchContext, applicationIssuer string)
	Role         func(element *etree.Element)
	PurposeOfUse func(element *etree.Element)
}

var applicationTokenType = TokenType{
	Subject: func(element *etree.Element, launchContext *LaunchContext, applicationIssuer string) {
		element.SetText(applicationIssuer)
	},
	Role: func(role *etree.Element) {
		role.CreateAttr("code", "182777000")
		role.CreateAttr("codeSystem", "2.16.840.1.113883.6.96")
		role.CreateAttr("codeSystemName", "SNOMED_CT")
		role.CreateAttr("displayName", "")
		role.CreateAttr("xmlns", "urn:hl7-org:v3")
	},
	PurposeOfUse: func(purposeOfUse *etree.Element) {
		purposeOfUse.CreateAttr("code", "OPERATIONS")
		purposeOfUse.CreateAttr("codeSystem", "2.16.840.1.113883.3.18.7.1")
		purposeOfUse.CreateAttr("codeSystemName", "nhin-purpose")
		purposeOfUse.CreateAttr("displayName", "")
		purposeOfUse.CreateAttr("xmlns", "urn:hl7-org:v3")
	},
}

var hcpTokenType = TokenType{
	Subject: func(element *etree.Element, launchContext *LaunchContext, applicationIssuer string) {
		element.SetText(*launchContext.Practitioner.Identifier[0].Value + "@" + *launchContext.Practitioner.Identifier[0].System)
	},
	Role: func(role *etree.Element) {
		role.CreateAttr("code", "224609002")
		role.CreateAttr("codeSystem", "2.16.840.1.113883.6.96")
		role.CreateAttr("codeSystemName", "SNOMED_CT")
		role.CreateAttr("displayName", "")
		role.CreateAttr("xmlns", "urn:hl7-org:v3")
	},
	PurposeOfUse: func(purposeOfUse *etree.Element) {
		purposeOfUse.CreateAttr("code", "TREATMENT")
		purposeOfUse.CreateAttr("codeSystem", "2.16.840.1.113883.3.18.7.1")
		purposeOfUse.CreateAttr("codeSystemName", "nhin-purpose")
		purposeOfUse.CreateAttr("displayName", "")
		purposeOfUse.CreateAttr("xmlns", "urn:hl7-org:v3")
	},
}

var _ SecureTokenService = &Service{}

// RequestAccessToken generates the SAML assertion, signs it, sends the SOAP request to the Zorgplatform STS and teturns the SAML access token
func (s *Service) RequestAccessToken(ctx context.Context, launchContext LaunchContext, tokenType TokenType) (string, error) {
	// Create the SAML assertion
	assertion, err := s.createSAMLAssertion(&launchContext, tokenType)

	if err != nil {
		return "Failed to create SAML assertion", err
	}

	// Sign the assertion
	signedAssertion, err := s.signAssertion(assertion)
	if err != nil {
		return "Failed to sign SAML assertion", err
	}

	// Wrap the signed assertion in a SOAP envelope
	envelope, err := s.createSOAPEnvelope(signedAssertion)
	if err != nil {
		return "Failed to create SOAP envelope", err
	}

	// Submit the request via mTLS
	response, err := s.submitSAMLRequest(ctx, envelope)
	if err != nil {
		return "Failed to submit SAML request", err
	}

	return s.validateRSTSResponse(response)
}

// createSAMLAssertion builds the SAML assertion
func (s *Service) createSAMLAssertion(launchContext *LaunchContext, tokenType TokenType) (*etree.Element, error) {
	assertionID := "_" + uuid.New().String()
	now := GetCurrentXSDDateTime()
	notOnOrAfter := FormatXSDDateTime(time.Now().Add(15 * time.Minute))

	// Create Assertion element
	assertion := etree.NewElement("Assertion")
	assertion.CreateAttr("ID", assertionID)
	assertion.CreateAttr("IssueInstant", now)
	assertion.CreateAttr("Version", "2.0")
	assertion.CreateAttr("xmlns", "urn:oasis:names:tc:SAML:2.0:assertion")

	// Issuer
	issuer := assertion.CreateElement("Issuer")
	issuer.SetText(s.config.SigningConfig.Issuer)

	// Subject
	subject := assertion.CreateElement("Subject")
	nameID := subject.CreateElement("NameID")
	tokenType.Subject(nameID, launchContext, s.config.SigningConfig.Issuer)

	subjectConfirmation := subject.CreateElement("SubjectConfirmation")
	subjectConfirmation.CreateAttr("Method", "urn:oasis:names:tc:SAML:2.0:cm:bearer")

	// Conditions
	conditions := assertion.CreateElement("Conditions")
	conditions.CreateAttr("NotBefore", now)
	conditions.CreateAttr("NotOnOrAfter", notOnOrAfter)
	audienceRestriction := conditions.CreateElement("AudienceRestriction")
	audience := audienceRestriction.CreateElement("Audience")
	audience.SetText(s.config.SigningConfig.Audience)

	// AttributeStatement
	attributeStatement := assertion.CreateElement("AttributeStatement")

	// PurposeOfUse Attribute
	attribute1 := attributeStatement.CreateElement("Attribute")
	attribute1.CreateAttr("Name", "urn:oasis:names:tc:xspa:1.0:subject:purposeofuse")
	attributeValue1 := attribute1.CreateElement("AttributeValue")
	purposeOfUse := attributeValue1.CreateElement("PurposeOfUse")
	tokenType.PurposeOfUse(purposeOfUse)

	// Role Attribute
	attribute2 := attributeStatement.CreateElement("Attribute")
	attribute2.CreateAttr("Name", "urn:oasis:names:tc:xacml:2.0:subject:role")
	attributeValue2 := attribute2.CreateElement("AttributeValue")
	role := attributeValue2.CreateElement("Role")
	tokenType.Role(role)

	// Resource ID Attribute
	attribute3 := attributeStatement.CreateElement("Attribute")
	attribute3.CreateAttr("Name", "urn:oasis:names:tc:xacml:1.0:resource:resource-id")
	attributeValue3 := attribute3.CreateElement("AttributeValue")
	instanceIdentifier := attributeValue3.CreateElement("InstanceIdentifier")
	instanceIdentifier.CreateAttr("root", "2.16.840.1.113883.2.4.6.3")
	instanceIdentifier.CreateAttr("extension", launchContext.Bsn)
	instanceIdentifier.CreateAttr("xmlns", "urn:hl7-org:v3")

	// Organization ID Attribute
	attribute4 := attributeStatement.CreateElement("Attribute")
	attribute4.CreateAttr("Name", "urn:oasis:names:tc:xspa:1.0:subject:organization-id")
	attributeValue4 := attribute4.CreateElement("AttributeValue")
	attributeValue4.SetText(s.config.SigningConfig.Issuer)

	// Workflow ID Attribute
	if launchContext.WorkflowId != "" {
		attribute5 := attributeStatement.CreateElement("Attribute")
		attribute5.CreateAttr("Name", zorgplatformWorkflowIdSystem)
		attributeValue5 := attribute5.CreateElement("AttributeValue")
		attributeValue5.SetText(launchContext.WorkflowId)
	}

	// AuthnStatement
	authnStatement := assertion.CreateElement("AuthnStatement")
	authnStatement.CreateAttr("AuthnInstant", now)
	authnContext := authnStatement.CreateElement("AuthnContext")
	authnContextClassRef := authnContext.CreateElement("AuthnContextClassRef")
	authnContextClassRef.SetText("urn:oasis:names:tc:SAML:2.0:ac:classes:X509")

	return assertion, nil
}

// signAssertion signs the SAML assertion
func (s *Service) signAssertion(assertion *etree.Element) (*etree.Element, error) {
	return SignAssertion(assertion, s.signingCertificateKey, s.signingCertificate)
}

func SignAssertion(assertion *etree.Element, signer crypto.Signer, certificatesRaw [][]byte) (*etree.Element, error) {
	// Step 1: Compute the Canonical Form of the Assertion Without the <Signature> Element

	// Make a deep copy of the assertion to avoid modifying the original during canonicalization
	assertionForDigest := assertion.Copy()

	// Canonicalize the assertion
	canonicalizer := dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	canonicalAssertion, err := canonicalizer.Canonicalize(assertionForDigest)
	if err != nil {
		return nil, err
	}

	// Step 2: Compute the Digest Value
	digest := sha256.Sum256(canonicalAssertion)
	digestValue := base64.StdEncoding.EncodeToString(digest[:])

	// Step 3: Build the <SignedInfo> Element
	signedInfo := buildSignedInfo(assertion.SelectAttrValue("ID", ""), digestValue)

	// Canonicalize the <SignedInfo> Element
	canonicalSignedInfo, err := canonicalizer.Canonicalize(signedInfo)
	if err != nil {
		return nil, err
	}

	// Step 4: Compute the Signature Value
	signatureBytes, err := signCanonicalizedSignedInfo(canonicalSignedInfo, signer)
	if err != nil {
		return nil, err
	}
	signatureValue := base64.StdEncoding.EncodeToString(signatureBytes)

	// Step 5: Construct the <Signature> Element
	signatureElement := etree.NewElement("Signature")
	signatureElement.CreateAttr("xmlns", "http://www.w3.org/2000/09/xmldsig#")
	signatureElement.AddChild(signedInfo)
	sigValueElement := signatureElement.CreateElement("SignatureValue")
	sigValueElement.SetText(signatureValue)
	keyInfo := signatureElement.CreateElement("KeyInfo")
	x509Data := keyInfo.CreateElement("X509Data")
	x509Certificate := x509Data.CreateElement("X509Certificate")
	x509Certificate.SetText(getCertificateBase64(certificatesRaw))

	// Step 6: Insert the <Signature> Element into the Assertion
	// Insert immediately after the <Issuer> element
	for idx, child := range assertion.ChildElements() {
		if child.Tag == "Issuer" {
			assertion.InsertChildAt(idx+1, signatureElement)
			break
		}
	}

	return assertion, nil
}

// Helper function to build the <SignedInfo> element
func buildSignedInfo(assertionID, digestValue string) *etree.Element {
	signedInfo := etree.NewElement("SignedInfo")
	signedInfo.CreateAttr("xmlns", "http://www.w3.org/2000/09/xmldsig#")

	canonicalizationMethod := signedInfo.CreateElement("CanonicalizationMethod")
	canonicalizationMethod.CreateAttr("Algorithm", "http://www.w3.org/2001/10/xml-exc-c14n#")

	signatureMethod := signedInfo.CreateElement("SignatureMethod")
	signatureMethod.CreateAttr("Algorithm", "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256")

	reference := signedInfo.CreateElement("Reference")
	reference.CreateAttr("URI", "#"+assertionID)

	transforms := reference.CreateElement("Transforms")
	transform1 := transforms.CreateElement("Transform")
	transform1.CreateAttr("Algorithm", "http://www.w3.org/2000/09/xmldsig#enveloped-signature")
	transform2 := transforms.CreateElement("Transform")
	transform2.CreateAttr("Algorithm", "http://www.w3.org/2001/10/xml-exc-c14n#")

	digestMethod := reference.CreateElement("DigestMethod")
	digestMethod.CreateAttr("Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")

	digestValueElement := reference.CreateElement("DigestValue")
	digestValueElement.SetText(digestValue)

	return signedInfo
}

// Helper function to sign the canonicalized SignedInfo
func signCanonicalizedSignedInfo(canonicalSignedInfo []byte, signer crypto.Signer) ([]byte, error) {
	// Compute the signature
	hash := sha256.Sum256(canonicalSignedInfo)

	signature, err := signer.Sign(rand.Reader, hash[:], crypto.SHA256)
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// Helper function to get the base64-encoded certificate
func getCertificateBase64(certificatesRaw [][]byte) string {
	combinedCertBytes := bytes.Join(certificatesRaw, nil)
	return base64.StdEncoding.EncodeToString(combinedCertBytes) //Only providing the Leaf works as well, safer?
}

// createSOAPEnvelope wraps the signed assertion in a SOAP envelope
func (s *Service) createSOAPEnvelope(signedAssertion *etree.Element) (string, error) {
	envelope := etree.NewElement("s:Envelope")
	envelope.CreateAttr("xmlns:s", "http://www.w3.org/2003/05/soap-envelope")
	envelope.CreateAttr("xmlns:a", "http://www.w3.org/2005/08/addressing")
	envelope.CreateAttr("xmlns:u", "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd")

	// SOAP Header
	header := envelope.CreateElement("s:Header")
	action := header.CreateElement("a:Action")
	action.CreateAttr("s:mustUnderstand", "1")
	action.SetText("http://docs.oasis-open.org/ws-sx/ws-trust/200512/RST/Issue")
	messageID := header.CreateElement("a:MessageID")
	messageID.SetText("urn:uuid:" + uuid.New().String())

	// ReplyTo
	replyTo := header.CreateElement("a:ReplyTo")
	address := replyTo.CreateElement("a:Address")
	address.SetText("http://www.w3.org/2005/08/addressing/anonymous")

	// To
	to := header.CreateElement("a:To")
	to.CreateAttr("s:mustUnderstand", "1")
	to.SetText(s.config.StsUrl)

	// Security element for the signed SAML assertion
	security := header.CreateElement("o:Security")
	security.CreateAttr("s:mustUnderstand", "1")
	security.CreateAttr("xmlns:o", "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd")

	// Timestamp
	timestamp := security.CreateElement("u:Timestamp")
	timestamp.CreateAttr("u:Id", "_0")
	created := timestamp.CreateElement("u:Created")

	now := time.Now()
	created.SetText(FormatXSDDateTime(now))
	expires := timestamp.CreateElement("u:Expires")
	expires.SetText(FormatXSDDateTime(now.Add(5 * time.Minute).UTC()))

	// Add the signed assertion to the security header
	security.AddChild(signedAssertion)

	// SOAP Body
	body := envelope.CreateElement("s:Body")
	rst := body.CreateElement("trust:RequestSecurityToken")
	rst.CreateAttr("xmlns:trust", "http://docs.oasis-open.org/ws-sx/ws-trust/200512")
	appliesTo := rst.CreateElement("wsp:AppliesTo")
	appliesTo.CreateAttr("xmlns:wsp", "http://schemas.xmlsoap.org/ws/2004/09/policy")
	endpointReference := appliesTo.CreateElement("wsa:EndpointReference")
	endpointReference.CreateAttr("xmlns:wsa", "http://www.w3.org/2005/08/addressing")
	rstAddress := endpointReference.CreateElement("wsa:Address")
	rstAddress.SetText(s.config.BaseUrl)
	keyType := rst.CreateElement("trust:KeyType")
	keyType.SetText("http://docs.oasis-open.org/ws-sx/ws-trust/200512/Bearer")
	requestType := rst.CreateElement("trust:RequestType")
	requestType.SetText("http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue")
	tokenType := rst.CreateElement("trust:TokenType")
	tokenType.SetText("http://docs.oasis-open.org/wss/oasis-wss-saml-token-profile-1.1#SAMLV2.0")

	// Convert the document to string
	doc := etree.NewDocument()
	doc.SetRoot(envelope)
	return doc.WriteToString()
}

// submitSAMLRequest sends the SOAP request over mTLS
func (s *Service) submitSAMLRequest(ctx context.Context, envelope string) (string, error) {
	tlsConfig := &tls.Config{
		Certificates:  []tls.Certificate{*s.tlsClientCertificate},
		MinVersion:    tls.VersionTLS12,
		Renegotiation: tls.RenegotiateOnceAsClient,
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, s.config.SAMLRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctxWithTimeout, "POST", s.config.StsUrl, bytes.NewBuffer([]byte(envelope)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read and return the response
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024*10)) //10mb
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		log.Ctx(ctxWithTimeout).Debug().Msgf("Zorgplatform STS SOAP request: %s", envelope)
		log.Ctx(ctxWithTimeout).Debug().Msgf("Zorgplatform STS SOAP response: %s", string(responseBody))
		return "", fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	return string(responseBody), nil
}

// validateRSTSResponse validates the generated Assertion and returns the SAML Bearer token from the RequestSecurityTokenResponse (RSTS)
func (s *Service) validateRSTSResponse(rtst string) (string, error) {
	doc := etree.NewDocument()
	err := doc.ReadFromString(rtst)
	if err != nil {
		return "", fmt.Errorf("failed to parse RTST SOAP response: %w", err)
	}

	assertionElement := doc.FindElement("//Assertion")
	if assertionElement == nil {
		return "", fmt.Errorf("assertion element not found in RTST response")
	}

	assertionDoc := etree.NewDocument()
	assertionDoc.SetRoot(assertionElement)
	assertionString, err := assertionDoc.WriteToString()
	if err != nil {
		return "", fmt.Errorf("failed to serialize RTST assertion: %w", err)
	}

	err = s.validateZorgplatformSignature(assertionElement)
	if err != nil {
		return "", fmt.Errorf("failed to validate RTST assertion signature: %w", err)
	}

	// Return the SAML Bearer token value; the base64 encoded <Assertion> element
	return base64.StdEncoding.EncodeToString([]byte(assertionString)), nil
}
