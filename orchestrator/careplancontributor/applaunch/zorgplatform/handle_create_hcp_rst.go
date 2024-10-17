package zorgplatform

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/google/uuid"
	"github.com/russellhaering/goxmldsig"
)

func (s *Service) RequestHcpRst(launchContext LaunchContext) (string, error) {

	assertion, err := s.createSAMLAssertion(&launchContext)
	if err != nil {
		return "", err
	}

	signedAssertion, err := s.signAssertion(assertion)
	if err != nil {
		return "", err
	}

	// Wrap the signed assertion in a SOAP envelope
	envelope := createSOAPEnvelope(signedAssertion)

	fmt.Println("SOAP req body: ")
	fmt.Println(envelope)

	// Submit the request via mTLS
	response, err := s.submitSAMLRequest(envelope)
	if err != nil {
		return "", err
	}

	return response, nil //TODO: Extract, validate & return the SAML token
}

// Sign the SAML assertion using goxmldsig
func (s *Service) signAssertion(assertion *saml.Assertion) (string, error) {

	doc := etree.NewDocument()
	assertionElement := assertion.Element()
	doc.SetRoot(assertionElement)

	// Create signing context with the private key
	signingContext, err := dsig.NewSigningContext(s.signingCertificateKey, [][]byte{s.signingCertificate.Raw})
	if err != nil {
		return "", err
	}

	// Set the canonicalizer for exclusive canonicalization with prefix list
	signingContext.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")

	// Sign the assertion with enveloped signature
	signedAssertion, err := signingContext.SignEnveloped(assertionElement)
	if err != nil {
		return "", err
	}

	// Convert signed assertion to string
	doc.SetRoot(signedAssertion)
	signedStr, err := doc.WriteToString()
	if err != nil {
		return "", err
	}

	return signedStr, nil
}

// Create the SAML assertion and wrap it in a SOAP envelope
func (s *Service) createSAMLAssertion(launchContext *LaunchContext) (*saml.Assertion, error) {

	audience := "https://zorgplatform.online"
	ID := uuid.New().String()

	assertion := &saml.Assertion{
		ID:           ID,
		IssueInstant: time.Now().UTC(),
		Version:      "2.0",
		Issuer: saml.Issuer{
			Value: s.config.OwnIssuer,
		},
		Subject: &saml.Subject{
			NameID: &saml.NameID{
				Value: *launchContext.Practitioner.Identifier[0].Value + "@" + *launchContext.Practitioner.Identifier[0].System,
			},
			SubjectConfirmations: []saml.SubjectConfirmation{
				{
					Method: "urn:oasis:names:tc:SAML:2.0:cm:bearer",
				},
			},
		},
		Conditions: &saml.Conditions{
			NotBefore:    time.Now().UTC(),
			NotOnOrAfter: time.Now().Add(15 * time.Minute).UTC(),
			AudienceRestrictions: []saml.AudienceRestriction{
				{Audience: saml.Audience{Value: audience}},
			},
		},
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: []saml.Attribute{
					{
						Name: "urn:oasis:names:tc:xspa:1.0:subject:purposeofuse",
						Values: []saml.AttributeValue{
							{Type: "xs:string", Value: `<PurposeOfUse code="TREATMENT" codeSystem="2.16.840.1.113883.3.18.7.1" xmlns="urn:hl7-org:v3"/>`},
						},
					},
					{
						Name: "urn:oasis:names:tc:xacml:2.0:subject:role",
						Values: []saml.AttributeValue{
							{Type: "xs:string", Value: `<Role code="158970007" codeSystem="2.16.840.1.113883.6.96" xmlns="urn:hl7-org:v3"/>`},
						},
					},
					{
						Name: "urn:oasis:names:tc:xacml:1.0:resource:resource-id",
						Values: []saml.AttributeValue{
							{Type: "xs:string", Value: `<InstanceIdentifier root="2.16.840.1.113883.2.4.6.3" extension="` + launchContext.Bsn + `" xmlns="urn:hl7-org:v3"/>`},
						},
					},
					{
						Name: "urn:oasis:names:tc:xspa:1.0:subject:organization-id",
						Values: []saml.AttributeValue{
							{Type: "xs:string", Value: "urn:oid:" + s.config.OwnIssuer},
						},
					},
					{
						Name: "http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id",
						Values: []saml.AttributeValue{
							{Type: "xs:string", Value: launchContext.WorkflowId},
						},
					},
				},
			},
		},
		AuthnStatements: []saml.AuthnStatement{
			{
				AuthnInstant: time.Now().UTC(),
				AuthnContext: saml.AuthnContext{
					AuthnContextClassRef: &saml.AuthnContextClassRef{
						Value: "urn:oasis:names:tc:SAML:2.0:ac:classes:X509",
					},
				},
			},
		},
	}

	return assertion, nil
}

func createSOAPEnvelope(signedAssertion string) string {
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

	// Security element for the signed SAML assertion
	security := header.CreateElement("o:Security")
	security.SetText(signedAssertion)

	// SOAP Body
	body := envelope.CreateElement("s:Body")
	rst := body.CreateElement("trust:RequestSecurityToken")
	rst.CreateAttr("xmlns:trust", "http://docs.oasis-open.org/ws-sx/ws-trust/200512")

	// Convert the document to string
	doc := etree.NewDocument()
	doc.SetRoot(envelope)
	str, _ := doc.WriteToString()
	return str
}

// Submit the signed SAML assertion over mTLS
func (s *Service) submitSAMLRequest(envelope string) (string, error) {

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*s.tlsClientCertificate},
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	req, err := http.NewRequest("POST", "https://zorgplatform.online/sts", bytes.NewBuffer([]byte(envelope)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.Header.Set("SOAPAction", "http://docs.oasis-open.org/ws-sx/ws-trust/200512/RST/Issue") //TODO: not in docs, but defined in wsdl

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read and return the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
