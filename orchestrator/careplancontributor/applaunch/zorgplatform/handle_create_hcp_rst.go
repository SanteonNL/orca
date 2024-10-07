package zorgplatform

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/google/uuid"
)

// requestHcpRst requests an HCP token from the Zorgplatform STS
func (s *Service) requestHcpRst(launchContext LaunchContext) (string, error) {
	// Create the SAML assertion
	assertion, err := s.createSAMLAssertion(launchContext)
	if err != nil {
		return "", err
	}

	// Sign the assertion using Azure Key Vault
	signedAssertion, err := s.signAssertion(assertion)
	if err != nil {
		return "", err
	}

	// Build the SOAP envelope
	soapEnvelope, err := s.buildSOAPEnvelope(signedAssertion)
	if err != nil {
		return "", err
	}

	// Serialize the SOAP envelope
	doc := etree.NewDocument()
	doc.SetRoot(soapEnvelope)
	soapXML, err := doc.WriteToString()
	if err != nil {
		return "", err
	}

	// Set up HTTPS client with mutual TLS
	//TODO: Load key (s.config.AzureConfig.KeyVaultConfig.TlsCertName) from Azure Key Vault
	tlsCert, err := tls.LoadX509KeyPair("TODO_REPLACE_WITH_AZURE_KV", "TODO_REPLACE_WITH_AZURE_KV")
	if err != nil {
		return "", err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{
		Transport: transport,
	}

	// Send the request
	req, err := http.NewRequest("POST", s.config.StsUrl, strings.NewReader(soapXML))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	fmt.Println("Response:", string(responseData))

	// TODO: Parse the response and extract the token
	return string(responseData), nil
}

// createSAMLAssertion creates a SAML 2.0 assertion based on the launch context
func (s *Service) createSAMLAssertion(launchContext LaunchContext) (*etree.Element, error) {
	//TODO: Check below for hard-coded values from the docs, we probably need to provide proper values for role etc

	assertion := etree.NewElement("Assertion")
	assertion.CreateAttr("xmlns", "urn:oasis:names:tc:SAML:2.0:assertion")
	assertionID := "_" + uuid.New().String()
	assertion.CreateAttr("ID", assertionID)
	assertion.CreateAttr("IssueInstant", time.Now().UTC().Format(time.RFC3339))
	assertion.CreateAttr("Version", "2.0")

	// Issuer
	issuer := assertion.CreateElement("Issuer")
	issuer.SetText(s.config.Issuer) // Replace with your HL7 OID, e.g., "urn:oid:YOUR_HL7_OID"

	// Subject
	subject := assertion.CreateElement("Subject")
	nameID := subject.CreateElement("NameID")
	nameID.SetText(launchContext.Subject) // Unique user ID
	subjectConfirmation := subject.CreateElement("SubjectConfirmation")
	subjectConfirmation.CreateAttr("Method", "urn:oasis:names:tc:SAML:2.0:cm:bearer")

	// Conditions
	conditions := assertion.CreateElement("Conditions")
	conditions.CreateAttr("NotBefore", time.Now().UTC().Format(time.RFC3339))
	conditions.CreateAttr("NotOnOrAfter", time.Now().Add(15*time.Minute).UTC().Format(time.RFC3339))
	audienceRestriction := conditions.CreateElement("AudienceRestriction")
	audience := audienceRestriction.CreateElement("Audience")
	audience.SetText(s.config.Audience) // "https://zorgplatform.online"

	// AttributeStatement
	attributeStatement := assertion.CreateElement("AttributeStatement")

	// PurposeOfUse
	attrPurposeOfUse := attributeStatement.CreateElement("Attribute")
	attrPurposeOfUse.CreateAttr("Name", "urn:oasis:names:tc:xspa:1.0:subject:purposeofuse")
	attrValuePurposeOfUse := attrPurposeOfUse.CreateElement("AttributeValue")
	purposeOfUse := attrValuePurposeOfUse.CreateElement("PurposeOfUse")
	purposeOfUse.CreateAttr("xmlns", "urn:hl7-org:v3")
	purposeOfUse.CreateAttr("code", "TREATMENT")
	purposeOfUse.CreateAttr("codeSystem", "2.16.840.1.113883.3.18.7.1")
	purposeOfUse.CreateAttr("codeSystemName", "nhin-purpose")

	// Role
	attrRole := attributeStatement.CreateElement("Attribute")
	attrRole.CreateAttr("Name", "urn:oasis:names:tc:xacml:2.0:subject:role")
	attrValueRole := attrRole.CreateElement("AttributeValue")
	role := attrValueRole.CreateElement("Role")
	role.CreateAttr("xmlns", "urn:hl7-org:v3")
	role.CreateAttr("code", "158970007") // SNOMED CT code for the role
	role.CreateAttr("codeSystem", "2.16.840.1.113883.6.96")
	role.CreateAttr("codeSystemName", "SNOMED_CT")

	// Resource ID (Patient BSN)
	attrResourceID := attributeStatement.CreateElement("Attribute")
	attrResourceID.CreateAttr("Name", "urn:oasis:names:tc:xacml:1.0:resource:resource-id")
	attrValueResourceID := attrResourceID.CreateElement("AttributeValue")
	instanceIdentifier := attrValueResourceID.CreateElement("InstanceIdentifier")
	instanceIdentifier.CreateAttr("xmlns", "urn:hl7-org:v3")
	instanceIdentifier.CreateAttr("root", "2.16.840.1.113883.2.4.6.3")
	instanceIdentifier.CreateAttr("extension", launchContext.Bsn)

	// Organization ID
	attrOrgID := attributeStatement.CreateElement("Attribute")
	attrOrgID.CreateAttr("Name", "urn:oasis:names:tc:xspa:1.0:subject:organization-id")
	attrValueOrgID := attrOrgID.CreateElement("AttributeValue")
	attrValueOrgID.SetText(s.config.Issuer)

	// Workflow ID (Optional)
	if launchContext.WorkflowId != "" {
		attrWorkflowID := attributeStatement.CreateElement("Attribute")
		attrWorkflowID.CreateAttr("Name", "http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id")
		attrValueWorkflowID := attrWorkflowID.CreateElement("AttributeValue")
		attrValueWorkflowID.SetText(launchContext.WorkflowId)
	} //TODO: Else throw error? We currently do not use application RSTs

	// AuthnStatement
	authnStatement := assertion.CreateElement("AuthnStatement")
	authnStatement.CreateAttr("AuthnInstant", time.Now().UTC().Format(time.RFC3339))
	authnContext := authnStatement.CreateElement("AuthnContext")
	authnContextClassRef := authnContext.CreateElement("AuthnContextClassRef")
	authnContextClassRef.SetText("urn:oasis:names:tc:SAML:2.0:ac:classes:X509")

	return assertion, nil
}

func (s *Service) signAssertion(assertion *etree.Element) (*etree.Element, error) {
	// 1. Canonicalize the Assertion (without the Signature element)
	canonicalizedAssertion, err := canonicalize(assertion)
	if err != nil {
		return nil, err
	}

	// 2. Compute the Digest of the Assertion
	hash := sha256.Sum256([]byte(canonicalizedAssertion))
	digestValue := base64.StdEncoding.EncodeToString(hash[:])

	// 3. Create the SignedInfo Element
	signedInfo := createSignedInfo(assertion.SelectAttrValue("ID", ""), digestValue)

	// 4. Canonicalize the SignedInfo
	canonicalizedSignedInfo, err := canonicalize(signedInfo)
	if err != nil {
		return nil, fmt.Errorf("canonicalization failed: %w", err)
	}

	// 5. Sign the Digest of SignedInfo
	signature, err := s.signingCertificate.SigningKey().Sign(rand.Reader, []byte(canonicalizedSignedInfo), crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %w", err)
	}

	// 6. Construct the Signature Element
	signatureElement, err := s.createSignatureElement(signedInfo, string(signature)) // TODO: Shouldn't this be base64 encoded?
	if err != nil {
		return nil, err
	}

	// 7. Embed the Signature in the Assertion
	assertion.AddChild(signatureElement)

	return assertion, nil
}

// canonicalize performs Exclusive XML Canonicalization on the provided element
func canonicalize(element *etree.Element) (string, error) {
	doc := etree.NewDocument()
	doc.SetRoot(element.Copy())
	canonicalXML, err := doc.WriteToString()
	if err != nil {
		return "", err
	}
	return string(canonicalXML), nil
}

// createSignedInfo constructs the SignedInfo element for the XML Signature
func createSignedInfo(assertionID, digestValue string) *etree.Element {
	signedInfo := etree.NewElement("SignedInfo")
	signedInfo.CreateAttr("xmlns", "http://www.w3.org/2000/09/xmldsig#")

	// CanonicalizationMethod
	canonicalizationMethod := signedInfo.CreateElement("CanonicalizationMethod")
	canonicalizationMethod.CreateAttr("Algorithm", "http://www.w3.org/2001/10/xml-exc-c14n#")

	// SignatureMethod
	signatureMethod := signedInfo.CreateElement("SignatureMethod")
	signatureMethod.CreateAttr("Algorithm", "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256")

	// Reference
	reference := signedInfo.CreateElement("Reference")
	reference.CreateAttr("URI", "#"+assertionID)

	// Transforms
	transforms := reference.CreateElement("Transforms")
	transform1 := transforms.CreateElement("Transform")
	transform1.CreateAttr("Algorithm", "http://www.w3.org/2000/09/xmldsig#enveloped-signature")
	transform2 := transforms.CreateElement("Transform")
	transform2.CreateAttr("Algorithm", "http://www.w3.org/2001/10/xml-exc-c14n#")

	// DigestMethod
	digestMethod := reference.CreateElement("DigestMethod")
	digestMethod.CreateAttr("Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")

	// DigestValue
	digestValueElement := reference.CreateElement("DigestValue")
	digestValueElement.SetText(digestValue)

	return signedInfo
}

// createSignatureElement constructs the Signature element
func (s *Service) createSignatureElement(signedInfo *etree.Element, signatureValue string) (*etree.Element, error) {
	signature := etree.NewElement("Signature")
	signature.CreateAttr("xmlns", "http://www.w3.org/2000/09/xmldsig#")
	signature.AddChild(signedInfo)

	// SignatureValue
	signatureValueElement := signature.CreateElement("SignatureValue")
	signatureValueElement.SetText(signatureValue)

	// KeyInfo
	keyInfo := signature.CreateElement("KeyInfo")
	x509Data := keyInfo.CreateElement("X509Data")
	x509Certificate := x509Data.CreateElement("X509Certificate")

	// Load your public certificate
	certData, err := os.ReadFile(s.config.AzureConfig.KeyVaultConfig.TlsCertName)
	if err != nil {
		return nil, err
	}
	certBlock := strings.ReplaceAll(string(certData), "-----BEGIN CERTIFICATE-----", "")
	certBlock = strings.ReplaceAll(certBlock, "-----END CERTIFICATE-----", "")
	certBlock = strings.ReplaceAll(certBlock, "\n", "")
	x509Certificate.SetText(certBlock)

	return signature, nil
}

// buildSOAPEnvelope constructs the SOAP envelope with the signed assertion
func (s *Service) buildSOAPEnvelope(signedAssertion *etree.Element) (*etree.Element, error) {
	soapEnvelope := etree.NewElement("s:Envelope")
	soapEnvelope.CreateAttr("xmlns:s", "http://www.w3.org/2003/05/soap-envelope")
	soapEnvelope.CreateAttr("xmlns:a", "http://www.w3.org/2005/08/addressing")
	soapEnvelope.CreateAttr("xmlns:u", "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd")

	// Header
	soapHeader := soapEnvelope.CreateElement("s:Header")

	// Action
	action := soapHeader.CreateElement("a:Action")
	action.CreateAttr("s:mustUnderstand", "1")
	action.SetText("http://docs.oasis-open.org/ws-sx/ws-trust/200512/RST/Issue")

	// MessageID
	messageID := soapHeader.CreateElement("a:MessageID")
	messageID.SetText("urn:uuid:" + uuid.New().String())

	// ReplyTo
	replyTo := soapHeader.CreateElement("a:ReplyTo")
	replyToAddress := replyTo.CreateElement("a:Address")
	replyToAddress.SetText("http://www.w3.org/2005/08/addressing/anonymous")

	// To
	to := soapHeader.CreateElement("a:To")
	to.CreateAttr("s:mustUnderstand", "1")
	to.SetText("https://zorgplatform.online/sts")

	// Security
	security := soapHeader.CreateElement("o:Security")
	security.CreateAttr("s:mustUnderstand", "1")
	security.CreateAttr("xmlns:o", "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd")

	// Timestamp
	timestamp := security.CreateElement("u:Timestamp")
	timestamp.CreateAttr("u:Id", "_0")
	created := timestamp.CreateElement("u:Created")
	created.SetText(time.Now().UTC().Format(time.RFC3339))
	expires := timestamp.CreateElement("u:Expires")
	expires.SetText(time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339))

	// Add the signed assertion to the security header
	security.AddChild(signedAssertion)

	// Body
	soapBody := soapEnvelope.CreateElement("s:Body")

	// RequestSecurityToken
	rst := soapBody.CreateElement("trust:RequestSecurityToken")
	rst.CreateAttr("xmlns:trust", "http://docs.oasis-open.org/ws-sx/ws-trust/200512")

	// AppliesTo
	appliesTo := rst.CreateElement("wsp:AppliesTo")
	appliesTo.CreateAttr("xmlns:wsp", "http://schemas.xmlsoap.org/ws/2004/09/policy")
	endpointReference := appliesTo.CreateElement("wsa:EndpointReference")
	endpointReference.CreateAttr("xmlns:wsa", "http://www.w3.org/2005/08/addressing")
	address := endpointReference.CreateElement("wsa:Address")
	address.SetText("https://zorgplatform.online/")

	// KeyType
	keyType := rst.CreateElement("trust:KeyType")
	keyType.SetText("http://docs.oasis-open.org/ws-sx/ws-trust/200512/Bearer")

	// RequestType
	requestType := rst.CreateElement("trust:RequestType")
	requestType.SetText("http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue")

	// TokenType
	tokenType := rst.CreateElement("trust:TokenType")
	tokenType.SetText("http://docs.oasis-open.org/wss/oasis-wss-saml-token-profile-1.1#SAMLV2.0")

	return soapEnvelope, nil
}
