package main

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/zorgplatform"
	"github.com/beevik/etree"
	"github.com/braineet/saml/xmlenc"
	"github.com/google/uuid"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 8 {
		panic("Usage: <bsn> <orgID> <audience> <userID> <workflowID> <encryption-cert-file> <launch-url>")
	}
	patientBSN := os.Args[1]
	orgID := os.Args[2]
	audience := os.Args[3]
	userID := os.Args[4]
	workflowID := os.Args[5]
	encryptionCertFile := os.Args[6]
	launchURL := os.Args[7]

	encryptionCertBytes, err := os.ReadFile(encryptionCertFile)
	if err != nil {
		panic(err)
	}
	var encryptionCert *x509.Certificate
	if strings.HasSuffix(encryptionCertFile, ".pem") {
		encryptionCertPEM, _ := pem.Decode(encryptionCertBytes)
		encryptionCert, err = x509.ParseCertificate(encryptionCertPEM.Bytes)
		if err != nil {
			panic(err)
		}
	} else if strings.HasSuffix(encryptionCertFile, ".cer") {
		decoded, err := base64.StdEncoding.DecodeString(string(encryptionCertBytes))
		if err != nil {
			decoded = encryptionCertBytes // fallback to raw bytes if base64 decoding fails
		}
		encryptionCert, err = x509.ParseCertificate(decoded)
		if err != nil {
			panic(err)
		}
	} else {
		panic("Unsupported encryption certificate file format. Use .pem or .cer")
	}

	signingCertificate, err := tls.LoadX509KeyPair("test-certificate.pem", "test-key.pem")
	if err != nil {
		panic(err)
	}
	samlResponse, err := createSAMLResponse(patientBSN, orgID, audience, userID, workflowID, signingCertificate, encryptionCert)
	if err != nil {
		panic(err)
	}
	println("SAML Response:")
	println("----------------------------------------")
	println(samlResponse)
	println("----------------------------------------")

	// Write an HTML file that contains the SAML response, and posts it
	htmlContent := `<!DOCTYPE html>
<html>
<head>
	<title>SAML Response</title>
</head>
<body>
	<form id="samlForm" method="POST" action="` + launchURL + `">
		<input type="hidden" name="SAMLResponse" value="` + samlResponse + `">
		<input type="submit" value="Submit SAML Response">
	</form>
	<script>
		document.getElementById('samlForm').submit();
	</script>
</body>
</html>`
	err = os.WriteFile("out/saml_response.html", []byte(htmlContent), 0644)
	if err != nil {
		panic(err)
	}
	println("HTML file with SAML response written to out/saml_response.html")
	println("You can open this file in a browser to submit the SAML response.")
}

func createSAMLResponse(patientBSN string, orgID string, audience string, userID string, workflowID string, signingCert tls.Certificate, encryptionKey *x509.Certificate) (string, error) {
	//const patientBSN = "999999102"
	//const organizationID = "2.16.840.1.113883.2.4.3.124.8.50.88"
	//const audience = "https://zorgplatform.test.integration.zorgbijjou.com/"
	//const userID = "682777"
	const issuer = "https://zorgplatform.online/sts"
	params := map[string]string{
		"PATIENT_BSN":  patientBSN,
		"ORG_ID":       orgID,
		"USER_ID":      userID,
		"ASSERTION_ID": uuid.NewString(),
		"ISSUER":       issuer,
		"AUDIENCE":     audience,
		// format as xs:dateTime
		"CURRENT_DATETIME":   time.Now().Format("2006-01-02T15:04:05Z07:00"),
		"NOTBEFORE_DATETIME": time.Now().Add(-1 * time.Minute).Format("2006-01-02T15:04:05Z07:00"),
		"NOTAFTER_DATETIME":  time.Now().Add(5 * time.Minute).Format("2006-01-02T15:04:05Z07:00"),
		"WORKFLOW_ID":        workflowID,
	}

	plainTextBytes, err := os.ReadFile("saml_assertion_input.xml")
	if err != nil {
		return "", err
	}
	plainText := string(plainTextBytes)

	for key, value := range params {
		// replace placeholders in the XML with actual values
		placeholder := "{{" + key + "}}"
		plainText = strings.ReplaceAll(plainText, placeholder, value)
	}
	println("Plain text SAML assertion:")
	println("----------------------------------------")
	println(plainText)
	println("----------------------------------------")
	assertionDoc := etree.NewDocument()
	err = assertionDoc.ReadFromBytes([]byte(plainText))
	if err != nil {
		return "", err
	}
	signedAssertion, err := zorgplatform.SignAssertion(assertionDoc.Root(), signingCert.PrivateKey.(crypto.Signer), signingCert.Certificate)
	if err != nil {
		return "", err
	}
	// Encrypt the signed assertion
	signedAssertionDoc := etree.NewDocument()
	signedAssertionDoc.AddChild(signedAssertion)
	plainText, err = signedAssertionDoc.WriteToString()
	if err != nil {
		return "", err
	}

	e := xmlenc.OAEP()
	e.BlockCipher = xmlenc.AES256CBC
	e.DigestMethod = &xmlenc.SHA1

	el, err := e.Encrypt(encryptionKey, []byte(plainText), nil)

	if err != nil {
		return "", err
	}
	samlResponse := etree.NewDocument()
	err = samlResponse.ReadFromFile("saml_response_input.xml")
	if err != nil {
		return "", err
	}

	// add encrypted assertion to RequestedSecurityToken element
	requestedSecurityToken := samlResponse.FindElement("//trust:RequestedSecurityToken/EncryptedAssertion")
	requestedSecurityToken.AddChild(el)

	samlResponseString, err := samlResponse.WriteToString()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString([]byte(samlResponseString)), nil
}
