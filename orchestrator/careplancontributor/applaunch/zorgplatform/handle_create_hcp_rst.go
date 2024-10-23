package zorgplatform

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	dsig "github.com/russellhaering/goxmldsig"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/google/uuid"
)

const timeFormat = "2006-01-02T15:04:05.999Z07:00"

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

	// Optional: You can also save the signed assertion to a file for easier verification with xmlsec1
	os.WriteFile("signed-envelope.xml", []byte(envelope), 0644)

	// Submit the request via mTLS
	response, err := s.submitSAMLRequest(envelope)
	if err != nil {
		return "", err
	}

	return response, nil //TODO: Extract, validate & return the SAML token
}

// Sign the SAML assertion using goxmldsig
func (s *Service) signAssertion(assertion *saml.Assertion) (*etree.Element, error) {

	doc := etree.NewDocument()
	assertionElement := assertion.Element()
	doc.SetRoot(assertionElement)

	// Create signing context with the private key
	signingContext, err := dsig.NewSigningContext(s.signingCertificateKey, s.signingCertificate)
	if err != nil {
		return nil, err
	}

	// Set the canonicalizer for exclusive canonicalization with prefix list
	signingContext.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")

	// Sign the assertion with enveloped signature
	signedAssertion, err := signingContext.SignEnveloped(assertionElement)
	if err != nil {
		return nil, err
	}
	// move Signature element right after Issuer
	signature := signedAssertion.SelectElement("Signature")
	for idx, element := range signedAssertion.ChildElements() {
		if element.Tag == "Signature" {
			signedAssertion.RemoveChildAt(idx)
		}
	}
	next := false
	for idx, element := range signedAssertion.ChildElements() {
		if next {
			signedAssertion.InsertChildAt(idx, signature)
			next = false
		}
		if element.Tag == "Issuer" {
			next = true
		}
	}

	// move saml:AuthnStatement element right after saml:AttributeStatement
	authnStatement := signedAssertion.SelectElement("saml:AuthnStatement")
	for idx, element := range signedAssertion.ChildElements() {
		if element.Tag == "AuthnStatement" {
			signedAssertion.RemoveChildAt(idx)
		}
	}
	next = false
	for idx, element := range signedAssertion.ChildElements() {
		if element.Tag == "AttributeStatement" {
			next = true
			signedAssertion.InsertChildAt(idx+1, authnStatement)
		}
	}

	return signedAssertion, nil
}

// Create the SAML assertion and wrap it in a SOAP envelope
func (s *Service) createSAMLAssertion(launchContext *LaunchContext) (*saml.Assertion, error) {

	audience := "https://zorgplatform.online"
	ID := uuid.New().String()

	purposeOfUse := etree.NewDocument()
	if err := purposeOfUse.ReadFromString(`<PurposeOfUse code="TREATMENT" codeSystem="2.16.840.1.113883.3.18.7.1" xmlns="urn:hl7-org:v3"/>`); err != nil {
		return nil, err
	}
	role := etree.NewDocument()
	if err := role.ReadFromString(`<Role code="158970007" codeSystem="2.16.840.1.113883.6.96" xmlns="urn:hl7-org:v3"/>`); err != nil {
		return nil, err
	}
	instanceIdentifier := etree.NewDocument()
	if err := instanceIdentifier.ReadFromString(`<InstanceIdentifier root="2.16.840.1.113883.2.4.6.3" extension="` + launchContext.Bsn + `" xmlns="urn:hl7-org:v3"/>`); err != nil {
		return nil, err
	}

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
				// Value: "USER1@2.16.840.1.113883.2.4.3.124.8.50.8",
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
							{XmlValue: purposeOfUse.ChildElements()[0]},
						},
					},
					{
						Name: "urn:oasis:names:tc:xacml:2.0:subject:role",
						Values: []saml.AttributeValue{
							{XmlValue: role.ChildElements()[0]},
						},
					},
					{
						Name: "urn:oasis:names:tc:xacml:1.0:resource:resource-id",
						Values: []saml.AttributeValue{
							{XmlValue: instanceIdentifier.ChildElements()[0]},
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

func createSOAPEnvelope(signedAssertion *etree.Element) string {
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
	//         <a:ReplyTo>
	//            <a:Address>http://www.w3.org/2005/08/addressing/anonymous</a:Address>
	//        </a:ReplyTo>
	replyTo := header.CreateElement("a:ReplyTo")
	address := replyTo.CreateElement("a:Address")
	address.SetText("http://www.w3.org/2005/08/addressing/anonymous")

	// To
	// <a:To s:mustUnderstand="1">https://zorgplatform.online/sts</a:To>
	to := header.CreateElement("a:To")
	to.CreateAttr("s:mustUnderstand", "1")
	to.SetText("https://zorgplatform.online/sts")

	// Security element for the signed SAML assertion
	security := header.CreateElement("u:Security")
	// MustUnderstand
	security.CreateAttr("s:mustUnderstand", "1")
	// Timestamp
	//             <u:Timestamp u:Id="_0">
	//                <u:Created>2019-04-19T12:55:23.030Z</u:Created>
	//                <u:Expires>2019-04-19T13:00:23.030Z</u:Expires>
	//            </u:Timestamp>
	timestamp := security.CreateElement("u:Timestamp")
	timestamp.CreateAttr("u:Id", "_0")
	created := timestamp.CreateElement("u:Created")
	created.SetText(time.Now().UTC().Format(timeFormat))
	expires := timestamp.CreateElement("u:Expires")
	expires.SetText(time.Now().Add(5 * time.Minute).UTC().Format(timeFormat))
	// Signature
	security.AddChild(signedAssertion)

	// SOAP Body
	//         <trust:RequestSecurityToken xmlns:trust="http://docs.oasis-open.org/ws-sx/ws-trust/200512">
	//            <wsp:AppliesTo xmlns:wsp="http://schemas.xmlsoap.org/ws/2004/09/policy">
	//                <wsa:EndpointReference xmlns:wsa="http://www.w3.org/2005/08/addressing">
	//                    <wsa:Address>[URL van web applicatie]</wsa:Address>
	//                </wsa:EndpointReference>
	//            </wsp:AppliesTo>
	//            <trust:KeyType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Bearer</trust:KeyType>
	//            <trust:RequestType>http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue</trust:RequestType>
	//            <trust:TokenType>http://docs.oasis-open.org/wss/oasis-wss-saml-token-profile-1.1#SAMLV2.0</trust:TokenType>
	//        </trust:RequestSecurityToken>
	body := envelope.CreateElement("s:Body")
	rst := body.CreateElement("trust:RequestSecurityToken")
	rst.CreateAttr("xmlns:trust", "http://docs.oasis-open.org/ws-sx/ws-trust/200512")
	appliesTo := rst.CreateElement("wsp:AppliesTo")
	appliesTo.CreateAttr("xmlns:wsp", "http://schemas.xmlsoap.org/ws/2004/09/policy")
	endpointReference := appliesTo.CreateElement("wsa:EndpointReference")
	endpointReference.CreateAttr("xmlns:wsa", "http://www.w3.org/2005/08/addressing")
	rstAddress := endpointReference.CreateElement("wsa:Address")
	rstAddress.SetText("https://zorgplatform.online")
	keyType := rst.CreateElement("trust:KeyType")
	keyType.SetText("http://docs.oasis-open.org/ws-sx/ws-trust/200512/Bearer")
	requestType := rst.CreateElement("trust:RequestType")
	requestType.SetText("http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue")
	tokenType := rst.CreateElement("trust:TokenType")
	tokenType.SetText("http://docs.oasis-open.org/wss/oasis-wss-saml-token-profile-1.1#SAMLV2.0")

	// Convert the document to string
	doc := etree.NewDocument()
	doc.SetRoot(envelope)
	str, _ := doc.WriteToString()
	return str
}

// Submit the signed SAML assertion over mTLS
func (s *Service) submitSAMLRequest(envelope string) (string, error) {

	tlsConfig := &tls.Config{
		Certificates:  []tls.Certificate{*s.tlsClientCertificate},
		Renegotiation: tls.RenegotiateFreelyAsClient,
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
	println("SOAP response body: ", string(body))

	// TODO: Move this before reading the body
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	return string(body), nil
}
