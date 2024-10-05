package zorgplatform

import (
	"encoding/base64"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"

	"github.com/beevik/etree"
	"github.com/rs/zerolog/log"
)

type LaunchContext struct {
	Bsn                string
	Subject            string
	WorkflowId         string
	DecryptedAssertion *etree.Document
}

func (s *Service) validateEncryptedSAMLToken(base64EncryptedToken string) (LaunchContext, error) {
	// TODO: Implement the SAML token validation logic
	log.Info().Msg("Validating encrypted SAML token")

	decodedEncryptedToken, err := base64.StdEncoding.DecodeString(base64EncryptedToken)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to decode base64 token: %w", err)
	}
	log.Info().Msgf("Decoded token: %s", decodedEncryptedToken)

	doc := etree.NewDocument()
	err = doc.ReadFromBytes(decodedEncryptedToken)

	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to parse XML: %w", err)
	}

	//TODO: Do we want to trim/cleanup values before validating?

	decryptedResponse, err := s.decryptAssertion(doc)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to decrypt assertion: %w", err)
	}

	log.Info().Msgf("Decrypted assertion: %s", string(decryptedResponse.Result)) //TODO: Remove this line, used for debugging

	decryptedAssertion := etree.NewDocument()
	err = doc.ReadFromBytes(decryptedResponse.Result)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to parse decrypted assertion XML: %w", err)
	}

	if err := s.validateTokenExpiry(decryptedAssertion); err != nil {
		return LaunchContext{}, fmt.Errorf("token has expired: %w", err)
	}

	// Validate the signature of the assertion using the public key of the Zorgplatform STS
	if err := s.validateSignature(decryptedAssertion); err != nil {
		return LaunchContext{}, fmt.Errorf("invalid assertion signature: %w", err)
	}

	// Validate the AudienceRestriction
	if err := s.validateAudience(decryptedAssertion); err != nil {
		return LaunchContext{}, fmt.Errorf("invalid audience: %w", err)
	}

	// // Validate the issuer
	if err := s.validateIssuer(decryptedAssertion); err != nil {
		return LaunchContext{}, fmt.Errorf("invalid issuer: %w", err)
	}

	// // Extract Subject/NameID and log in the user
	subject, err := s.extractSubject(decryptedAssertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract subject: %w", err)
	}

	// Extract resource-id claim to select the correct patient
	resourceID, err := s.extractResourceID(decryptedAssertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract resource-id: %w", err)
	}

	workflowID, err := s.extractWorkflowID(decryptedAssertion)
	if err != nil {
		return LaunchContext{}, fmt.Errorf("unable to extract workflow-id: %w", err)
	}

	// // Process any other required attributes (claims)
	// if err := s.processAdditionalAttributes(decryptedAssertion); err != nil {
	// 	return fmt.Errorf("unable to process additional attributes: %w", err)
	// }

	return LaunchContext{
		Bsn:                resourceID,
		Subject:            subject,
		WorkflowId:         workflowID,
		DecryptedAssertion: decryptedAssertion,
	}, nil

}

func (s *Service) decryptAssertion(doc *etree.Document) (azkeys.DecryptResponse, error) {
	// privateKey, err := loadPrivateKey()

	el := doc.Root().FindElement("//EncryptedAssertion/xenc:EncryptedData/xenc:CipherData/xenc:CipherValue")
	cipher := el.Text()

	//TODO: Currently we only support azure keyvault decryption, ideally this is configurable and logic is extended here
	decryptedValue, err := azkeyvault.Decrypt([]byte(cipher), s.config.AzureConfig.KeyVaultConfig.KeyVaultURL, s.config.AzureConfig.KeyVaultConfig.DecryptCertName, s.config.AzureConfig.KeyVaultConfig.DecryptCertVersion)

	// result, err := xmlenc.Decrypt(privateKey, el)
	if err != nil {
		return azkeys.DecryptResponse{}, fmt.Errorf("failed to decrypt XML: %v", err)
	}

	return decryptedValue, nil
}

func (s *Service) validateTokenExpiry(doc *etree.Document) error {
	createdElement := doc.Root().FindElement("//u:Timestamp/u:Created")
	expiresElement := doc.Root().FindElement("//u:Timestamp/u:Expires")

	if createdElement == nil || expiresElement == nil {
		return fmt.Errorf("timestamp elements not found in the assertion")
	}

	created := createdElement.Text()
	expires := expiresElement.Text()

	log.Trace().Msgf("Token created: %s", created)
	log.Trace().Msgf("Token expires: %s", expires)

	createdTime, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return fmt.Errorf("invalid created time format: %w", err)
	}

	expiresTime, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return fmt.Errorf("invalid expires time format: %w", err)
	}

	now := time.Now().UTC()
	if now.Before(createdTime) || now.After(expiresTime) {
		return fmt.Errorf("token is not valid at the current time")
	}

	return nil
}

// Validates the descrypted assertion signature using the public key of the Zorgplatform STS
func (s *Service) validateSignature(decryptedAssertion *etree.Document) error {
	signatureElement := decryptedAssertion.Root().FindElement("//Signature")
	if signatureElement == nil {
		return fmt.Errorf("signature element not found in the assertion")
	}

	//TODO: Implement when we can decrypt the assertions

	return nil
}

func (s *Service) validateAudience(decryptedAssertion *etree.Document) error {
	el := decryptedAssertion.Root().FindElement("//AudienceRestriction/Audience")
	aud := el.Text()

	if aud == s.config.Audience {
		return nil
	}

	return fmt.Errorf("invalid aud. Found [%s] but expected [%s]", aud, s.config.Audience)
}

func (s *Service) validateIssuer(decryptedAssertion *etree.Document) error {

	el := decryptedAssertion.Root().FindElement("//Issuer")
	iss := el.Text()

	if iss == s.config.Issuer {
		return nil
	}
	return fmt.Errorf("invalid iss. Found [%s] but expected [%s]", iss, s.config.Issuer)
}

func (s *Service) extractSubject(decryptedAssertion *etree.Document) (string, error) {

	el := decryptedAssertion.Root().FindElement("//Subject/NameID")
	sub := el.Text()

	if sub != "" {
		return sub, nil
	}

	return "", fmt.Errorf("could not find sub of assertion")
}

func (s *Service) extractResourceID(decryptedAssertion *etree.Document) (string, error) {
	// Find the Attribute element with the specified Name
	attributeElement := decryptedAssertion.Root().FindElement("//Attribute[@Name='urn:oasis:names:tc:xacml:1.0:resource:resource-id']/AttributeValue/InstanceIdentifier")
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

func (s *Service) extractWorkflowID(decryptedAssertion *etree.Document) (string, error) {
	workflowIdElement := decryptedAssertion.Root().FindElement("//Attribute[@Name='http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id']/AttributeValue")
	if workflowIdElement == nil {
		return "", fmt.Errorf("workflow-id attribute not found in the assertion")
	}
	workflowId := workflowIdElement.Text()
	log.Debug().Msgf("Extracted workflow-id: %s", workflowId)

	return workflowId, nil
}

func (s *Service) processAdditionalAttributes(decryptedAssertion *etree.Document) error {
	// TODO: Implement the logic to process additional attributes (claims) from the assertion
	// This is a placeholder implementation
	return nil
}
