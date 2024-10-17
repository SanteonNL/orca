package zorgplatform

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/beevik/etree"
	"github.com/google/uuid"
	"github.com/russellhaering/goxmldsig"
)

// SAMLAssertionData holds the data for the SAML assertion template
type SAMLAssertionData struct {
	AssertionID    string
	IssueInstant   string
	Issuer         string
	NameID         string
	NotBefore      string
	NotOnOrAfter   string
	Audience       string
	PatientBSN     string
	OrganizationID string
	WorkflowID     string
	AuthnInstant   string
	DigestValue    string
	SignatureValue string
	Certificates   []string
}

// SOAPEnvelopeData holds the data for the SOAP envelope template
type SOAPEnvelopeData struct {
	MessageID       string
	STSURL          string
	Created         string
	Expires         string
	AppliesTo       string
	SignedAssertion string
}

// SAML assertion template with placeholders for DigestValue, SignatureValue, and the X509Certificate
const samlAssertionTemplate = `
<Assertion ID="{{.AssertionID}}" IssueInstant="{{.IssueInstant}}" Version="2.0" xmlns="urn:oasis:names:tc:SAML:2.0:assertion">
	<Issuer>{{.Issuer}}</Issuer>
	<Signature xmlns="http://www.w3.org/2000/09/xmldsig#">
		<SignedInfo>
			<CanonicalizationMethod Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#" />
			<SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256" />
			<Reference URI="#{{.AssertionID}}">
				<Transforms>
					<Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature" />
					<Transform Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#" />
				</Transforms>
				<DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256" />
				<DigestValue>{{.DigestValue}}</DigestValue>
			</Reference>
		</SignedInfo>
		<SignatureValue>{{.SignatureValue}}</SignatureValue>
		<KeyInfo>
			<X509Data>
				{{range .Certificates}}
				<X509Certificate>{{.}}</X509Certificate>
				{{end}}
			</X509Data>
		</KeyInfo>
	</Signature>
	<Subject>
		<NameID>{{.NameID}}</NameID>
		<SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer" />
	</Subject>
	<Conditions NotBefore="{{.NotBefore}}" NotOnOrAfter="{{.NotOnOrAfter}}">
		<AudienceRestriction>
			<Audience>{{.Audience}}</Audience>
		</AudienceRestriction>
	</Conditions>
	<AttributeStatement>
		<Attribute Name="urn:oasis:names:tc:xspa:1.0:subject:purposeofuse">
			<AttributeValue>
				<PurposeOfUse code="TREATMENT" codeSystem="2.16.840.1.113883.3.18.7.1"
					codeSystemName="nhin-purpose" 
					displayName="" xmlns="urn:hl7-org:v3" />
			</AttributeValue>
		</Attribute>
		<Attribute Name="urn:oasis:names:tc:xacml:2.0:subject:role">
			<AttributeValue>
				<Role code="158970007" codeSystem="2.16.840.1.113883.6.96"
					codeSystemName="SNOMED_CT" displayName=""
					xmlns="urn:hl7-org:v3" />
			</AttributeValue>
		</Attribute>
		<Attribute Name="urn:oasis:names:tc:xacml:1.0:resource:resource-id">
			<AttributeValue>
				<InstanceIdentifier root="2.16.840.1.113883.2.4.6.3"
					extension="{{.PatientBSN}}" xmlns="urn:hl7-org:v3" />
			</AttributeValue>
		</Attribute>
		<Attribute Name="urn:oasis:names:tc:xspa:1.0:subject:organization-id">
			<AttributeValue>{{.OrganizationID}}</AttributeValue>
		</Attribute>
		<Attribute Name="http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id">
			<AttributeValue>{{.WorkflowID}}</AttributeValue>
		</Attribute>
	</AttributeStatement>
	<AuthnStatement AuthnInstant="{{.AuthnInstant}}">
		<AuthnContext>
			<AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:X509</AuthnContextClassRef>
		</AuthnContext>
	</AuthnStatement>
</Assertion>`

// SOAP envelope template with a placeholder for the signed SAML assertion
const soapEnvelopeTemplate = `
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing" xmlns:u="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd">
    <s:Header>
        <a:Action s:mustUnderstand="1">http://docs.oasis-open.org/ws-sx/ws-trust/200512/RST/Issue</a:Action>
        <a:MessageID>{{.MessageID}}</a:MessageID>
        <a:ReplyTo>
            <a:Address>http://www.w3.org/2005/08/addressing/anonymous</a:Address>
        </a:ReplyTo>
        <a:To s:mustUnderstand="1">{{.STSURL}}</a:To>
        <o:Security s:mustUnderstand="1" xmlns:o="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd">
            <u:Timestamp u:Id="_0">
                <u:Created>{{.Created}}</u:Created>
                <u:Expires>{{.Expires}}</u:Expires>
            </u:Timestamp>
{{.SignedAssertion}}
        </o:Security>
    </s:Header>
    <s:Body>
        <trust:RequestSecurityToken xmlns:trust="http://docs.oasis-open.org/ws-sx/ws-trust/200512">
            <wsp:AppliesTo xmlns:wsp="http://schemas.xmlsoap.org/ws/2004/09/policy">
                <wsa:EndpointReference xmlns:wsa="http://www.w3.org/2005/08/addressing">
                    <wsa:Address>{{.AppliesTo}}</wsa:Address>
                </wsa:EndpointReference>
            </wsp:AppliesTo>
            <trust:KeyType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Bearer</trust:KeyType>
            <trust:RequestType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue</trust:RequestType>
            <trust:TokenType>http://docs.oasis-open.org/wss/oasis-wss-saml-token-profile-1.1#SAMLV2.0</trust:TokenType>
        </trust:RequestSecurityToken>
    </s:Body>
</s:Envelope>`

// RequestHcpRst generates the SAML assertion, signs it, and sends the SOAP request to the Zorgplatform STS
func (s *Service) RequestHcpRst(launchContext LaunchContext) (string, error) {
	// Create and sign the SAML assertion
	signedAssertion, err := s.createAndSignAssertion(launchContext)
	if err != nil {
		return "", err
	}

	// fmt.Println(signedAssertion)

	// Build the SOAP envelope
	soapEnvelope, err := s.buildSOAPEnvelope(signedAssertion)
	if err != nil {
		return "", err
	}

	fmt.Println("SOAP req body: ")
	fmt.Println(soapEnvelope)

	// Set up HTTPS client with mutual TLS
	tlsConfig := &tls.Config{
		Certificates:  []tls.Certificate{*s.tlsClientCertificate},
		MinVersion:    tls.VersionTLS12,
		Renegotiation: tls.RenegotiateOnceAsClient,
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{
		Transport: transport,
	}

	// Send the request
	req, err := http.NewRequest("POST", s.config.StsUrl, strings.NewReader(soapEnvelope))
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

	return string(responseData), nil
}

// createAndSignAssertion fills the template, canonicalizes it, computes the digest, and signs the assertion
func (s *Service) createAndSignAssertion(launchContext LaunchContext) (string, error) {
	assertionData := SAMLAssertionData{
		AssertionID:    "_" + uuid.New().String(),
		IssueInstant:   time.Now().UTC().Format(time.RFC3339),
		Issuer:         s.config.OwnIssuer,
		NameID:         *launchContext.Practitioner.Identifier[0].Value + "@" + *launchContext.Practitioner.Identifier[0].System,
		NotBefore:      time.Now().UTC().Format(time.RFC3339),
		NotOnOrAfter:   time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
		Audience:       "https://zorgplatform.online",
		PatientBSN:     launchContext.Bsn,
		OrganizationID: s.config.OwnIssuer,
		WorkflowID:     launchContext.WorkflowId,
		AuthnInstant:   time.Now().UTC().Format(time.RFC3339Nano),
	}

	// Fill the assertion template without the DigestValue and SignatureValue
	var assertionBuf bytes.Buffer
	tmpl, err := template.New("assertion").Parse(samlAssertionTemplate)
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(&assertionBuf, assertionData)
	if err != nil {
		return "", err
	}

	// Canonicalize the assertion for digest calculation (excluding the <Signature>)
	canonicalAssertion, err := s.canonicalize(assertionBuf.Bytes())
	if err != nil {
		return "", err
	}

	// Compute the digest value
	digestValue := s.computeDigest(canonicalAssertion)
	assertionData.DigestValue = digestValue

	// Canonicalize SignedInfo and sign it
	canonicalSignedInfo, err := s.canonicalizeSignedInfo(assertionData.AssertionID, digestValue)
	if err != nil {
		return "", err
	}
	signatureValue, err := s.signCanonicalizedSignedInfo(canonicalSignedInfo)
	if err != nil {
		return "", err
	}
	assertionData.SignatureValue = signatureValue

	// Load the certificate
	certData, err := s.loadCertificates()
	if err != nil {
		return "", err
	}
	assertionData.Certificates = certData

	// Fill the assertion template with DigestValue, SignatureValue, and Certificate
	var signedAssertionBuf bytes.Buffer
	err = tmpl.Execute(&signedAssertionBuf, assertionData)
	if err != nil {
		return "", err
	}

	return signedAssertionBuf.String(), nil
}

// buildSOAPEnvelope builds the SOAP envelope around the signed SAML assertion
func (s *Service) buildSOAPEnvelope(signedAssertion string) (string, error) {
	soapData := SOAPEnvelopeData{
		MessageID:       "urn:uuid:" + uuid.New().String(),
		STSURL:          "https://zorgplatform.online/sts",
		Created:         time.Now().UTC().Format(time.RFC3339),
		Expires:         time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
		AppliesTo:       "https://zorgplatform.online/",
		SignedAssertion: signedAssertion,
	}

	// Fill the SOAP envelope template
	var soapBuf bytes.Buffer
	tmpl, err := template.New("soapEnvelope").Parse(soapEnvelopeTemplate)
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(&soapBuf, soapData)
	if err != nil {
		return "", err
	}

	return soapBuf.String(), nil
}

// canonicalize performs Exclusive XML Canonicalization on the provided XML
func (s *Service) canonicalize(xmlData []byte) ([]byte, error) {
	ctx := dsig.NewDefaultSigningContext(nil)
	ctx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlData); err != nil {
		return nil, err
	}
	canonicalized, err := ctx.Canonicalizer.Canonicalize(doc.Root())
	if err != nil {
		return nil, err
	}

	fmt.Println("c14n output: ")
	fmt.Println(string(canonicalized))

	return canonicalized, nil
}

// computeDigest computes the SHA-256 digest of the canonicalized assertion
func (s *Service) computeDigest(assertion []byte) string {
	hash := sha256.Sum256(assertion)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// canonicalizeSignedInfo creates and canonicalizes the SignedInfo for signing
func (s *Service) canonicalizeSignedInfo(assertionID, digestValue string) ([]byte, error) {
	signedInfo := fmt.Sprintf(`
<SignedInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
    <CanonicalizationMethod Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#" />
    <SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256" />
    <Reference URI="#%s">
        <Transforms>
            <Transform Algorithm="http://www.w3.org/2000/09/xmldsig#enveloped-signature" />
            <Transform Algorithm="http://www.w3.org/2001/10/xml-exc-c14n#" />
        </Transforms>
        <DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256" />
        <DigestValue>%s</DigestValue>
    </Reference>
</SignedInfo>`, assertionID, digestValue)

	// Canonicalize SignedInfo
	return s.canonicalize([]byte(signedInfo))
}

// signCanonicalizedSignedInfo signs the canonicalized SignedInfo and returns the signature value
func (s *Service) signCanonicalizedSignedInfo(canonicalSignedInfo []byte) (string, error) {
	// Load the private key
	privateKeyData, err := os.ReadFile(s.config.X509FileConfig.SignKeyFile)
	if err != nil {
		return "", fmt.Errorf("unable to read sign key from file: %w", err)
	}

	block, _ := pem.Decode(privateKeyData)
	if block == nil || block.Type != "PRIVATE KEY" {
		return "", fmt.Errorf("failed to decode PEM block containing private key")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("not an RSA private key")
	}

	// Sign the canonicalized SignedInfo
	signatureHash := sha256.Sum256(canonicalSignedInfo)
	signatureBytes, err := rsaPrivateKey.Sign(rand.Reader, signatureHash[:], crypto.SHA256)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signatureBytes), nil
}

// loadCertificate loads the X509 certificate from file and returns the base64-encoded certificate string
func (s *Service) loadCertificates() ([]string, error) {
	certData, err := os.ReadFile(s.config.X509FileConfig.SignCertFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read sign certificate from file: %w", err)
	}

	var certificates []string
	for {
		block, rest := pem.Decode(certData)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			certificates = append(certificates, base64.StdEncoding.EncodeToString(block.Bytes))
		}
		certData = rest
	}

	if len(certificates) == 0 {
		return nil, fmt.Errorf("no certificates found in PEM data")
	}

	return certificates, nil
}
