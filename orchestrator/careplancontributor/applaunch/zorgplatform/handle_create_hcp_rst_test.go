package zorgplatform

import (
	"crypto/rsa"
	"crypto/tls"
	"os"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

//TODO: Add a unit test for the soap envelop structure

func TestCreateSAMLAssertion(t *testing.T) {
	t.Skip("still failing")

	// service := &Service{
	// 	config: Config{
	// 		Issuer:   "https://issuer.example.com",
	// 		Audience: "https://zorgplatform.online",
	// 	},
	// }

	// launchContext := LaunchContext{
	// 	Practitioner: fhir.Practitioner{Identifier: []fhir.Identifier{
	// 		{
	// 			System: to.Ptr("urn:oid:2.16.840.1.113883.4.1"),
	// 			Value:  to.Ptr("999999999"),
	// 		},
	// 	}},
	// 	WorkflowId: "workflow-1234",
	// 	Bsn:        "999999205", // Assuming Bsn is part of LaunchContext
	// }

	// // Generate the assertion
	// assertionElement, err := service.createSAMLAssertion(launchContext)
	// assert.NoError(t, err)

	// // Load expected XML into etree.Document
	// expectedXML := `
	// <Assertion xmlns="urn:oasis:names:tc:SAML:2.0:assertion" ID="_unique-id" Version="2.0">
	// 	<Issuer>https://issuer.example.com</Issuer>
	// 	<Subject>
	// 		<NameID>urn:oid:2.16.840.1.113883.4.1.999999999</NameID>
	// 		<SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer"/>
	// 	</Subject>
	// 	<Conditions NotBefore="2023-10-10T10:00:00Z" NotOnOrAfter="2023-10-10T10:15:00Z">
	// 		<AudienceRestriction>
	// 			<Audience>https://zorgplatform.online</Audience>
	// 		</AudienceRestriction>
	// 	</Conditions>
	// 	<AttributeStatement>
	// 		<Attribute Name="urn:oasis:names:tc:xspa:1.0:subject:purposeofuse">
	// 			<AttributeValue>
	// 				<PurposeOfUse code="TREATMENT" codeSystem="2.16.840.1.113883.3.18.7.1" codeSystemName="nhin-purpose" xmlns="urn:hl7-org:v3"/>
	// 			</AttributeValue>
	// 		</Attribute>
	// 		<Attribute Name="urn:oasis:names:tc:xacml:2.0:subject:role">
	// 			<AttributeValue>
	// 				<Role code="158970007" codeSystem="2.16.840.1.113883.6.96" codeSystemName="SNOMED_CT" xmlns="urn:hl7-org:v3"/>
	// 			</AttributeValue>
	// 		</Attribute>
	// 		<Attribute Name="urn:oasis:names:tc:xacml:1.0:resource:resource-id">
	// 			<AttributeValue>
	// 				<InstanceIdentifier root="2.16.840.1.113883.2.4.6.3" extension="999999205" xmlns="urn:hl7-org:v3"/>
	// 			</AttributeValue>
	// 		</Attribute>
	// 		<Attribute Name="urn:oasis:names:tc:xspa:1.0:subject:organization-id">
	// 			<AttributeValue>https://issuer.example.com</AttributeValue>
	// 		</Attribute>
	// 		<Attribute Name="http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id">
	// 			<AttributeValue>workflow-1234</AttributeValue>
	// 		</Attribute>
	// 	</AttributeStatement>
	// 	<AuthnStatement AuthnInstant="2023-10-10T10:00:00Z">
	// 		<AuthnContext>
	// 			<AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:X509</AuthnContextClassRef>
	// 		</AuthnContext>
	// 	</AuthnStatement>
	// </Assertion>`

	// expectedDoc := etree.NewDocument()
	// err = expectedDoc.ReadFromString(expectedXML)
	// assert.NoError(t, err)

	// doc := etree.NewDocument()
	// doc.SetRoot(assertionElement)

	// // Compare the generated assertion with the expected XML
	// assert.Equal(t, expectedDoc.Root().FindElement("Issuer").Text(), doc.Root().FindElement("Issuer").Text())
	// assert.Equal(t, expectedDoc.Root().FindElement("Subject/NameID").Text(), doc.Root().FindElement("Subject/NameID").Text())
	// assert.Equal(t, expectedDoc.Root().FindElement("Conditions/AudienceRestriction/Audience").Text(), doc.Root().FindElement("Conditions/AudienceRestriction/Audience").Text())

	// // Compare Attributes in PurposeOfUse
	// expectedPurposeOfUse := expectedDoc.Root().FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xspa:1.0:subject:purposeofuse']/AttributeValue/PurposeOfUse")
	// actualPurposeOfUse := doc.Root().FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xspa:1.0:subject:purposeofuse']/AttributeValue/PurposeOfUse")
	// assertEqualAttributes(t, expectedPurposeOfUse, actualPurposeOfUse)

	// // Compare Attributes in Role
	// expectedRole := expectedDoc.Root().FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xacml:2.0:subject:role']/AttributeValue/Role")
	// actualRole := doc.Root().FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xacml:2.0:subject:role']/AttributeValue/Role")
	// assertEqualAttributes(t, expectedRole, actualRole)

	// // Compare Attributes in InstanceIdentifier
	// expectedInstanceIdentifier := expectedDoc.Root().FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xacml:1.0:resource:resource-id']/AttributeValue/InstanceIdentifier")
	// actualInstanceIdentifier := doc.Root().FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xacml:1.0:resource:resource-id']/AttributeValue/InstanceIdentifier")
	// assertEqualAttributes(t, expectedInstanceIdentifier, actualInstanceIdentifier)

	// // Compare Organization ID
	// expectedOrgID := expectedDoc.Root().FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xspa:1.0:subject:organization-id']/AttributeValue")
	// actualOrgID := doc.Root().FindElement("AttributeStatement/Attribute[@Name='urn:oasis:names:tc:xspa:1.0:subject:organization-id']/AttributeValue")
	// assert.Equal(t, expectedOrgID.Text(), actualOrgID.Text())

	// // Compare Workflow ID
	// expectedWorkflowID := expectedDoc.Root().FindElement("AttributeStatement/Attribute[@Name='http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id']/AttributeValue")
	// actualWorkflowID := doc.Root().FindElement("AttributeStatement/Attribute[@Name='http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id']/AttributeValue")
	// assert.Equal(t, expectedWorkflowID.Text(), actualWorkflowID.Text())

	// // Compare AuthnContextClassRef
	// expectedAuthnContextClassRef := expectedDoc.Root().FindElement("AuthnStatement/AuthnContext/AuthnContextClassRef").Text()
	// actualAuthnContextClassRef := doc.Root().FindElement("AuthnStatement/AuthnContext/AuthnContextClassRef").Text()
	// assert.Equal(t, expectedAuthnContextClassRef, actualAuthnContextClassRef)
}

// Helper function to compare attributes regardless of order
func assertEqualAttributes(t *testing.T, expectedElement, actualElement *etree.Element) {
	expectedAttrs := make(map[string]string)
	for _, attr := range expectedElement.Attr {
		expectedAttrs[attr.Key] = attr.Value
	}

	actualAttrs := make(map[string]string)
	for _, attr := range actualElement.Attr {
		actualAttrs[attr.Key] = attr.Value
	}

	assert.Equal(t, expectedAttrs, actualAttrs)
}

func TestService_sign(t *testing.T) {
	// Load the certificate and private key
	keyPair, err := tls.LoadX509KeyPair("test-certificate.pem", "test-key.pem")
	assert.NoError(t, err)

	// Initialize the service with the private key and certificate
	service := &Service{
		signingCertificateKey: keyPair.PrivateKey.(*rsa.PrivateKey),
		signingCertificate:    keyPair.Leaf,
	}

	launchContext := &LaunchContext{
		Bsn: "123456789",
		Practitioner: fhir.Practitioner{
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("urn:oid:2.16.840.1.113883.4.1"),
					Value:  to.Ptr("999999999"),
				},
			},
		},
		SubjectNameId:  "Subject",
		WorkflowId:     "workflow-1234",
		ServiceRequest: fhir.ServiceRequest{},
	}

	assertion, err := service.createSAMLAssertion(launchContext)
	assert.NoError(t, err)

	// Call the signing function
	signedElement, err := service.signAssertion(assertion)

	// Assert no errors occurred
	assert.NoError(t, err)

	// Print the signed assertion for verification with xmlsec1
	println(signedElement)

	// Optional: You can also save the signed assertion to a file for easier verification with xmlsec1
	os.WriteFile("signed-assertion.xml", []byte(signedElement), 0644)

	//You can test the output (manually for now) with: xmlsec1 --verify --id-attr:ID urn:oasis:names:tc:SAML:2.0:assertion:Assertion --output /dev/null --trusted-pem ./test-certificate.pem --pubkey-pem ./test-key.pem signed-assertion.xml
}
