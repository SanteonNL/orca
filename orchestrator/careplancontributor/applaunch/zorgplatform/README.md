# Zorgplatform Integration

## Field Mapping

This section describes how fields from ChipSoft HiX (ChipSoft's EHR), Zorgplatform (ChipSoft's API to access data from
HiX in FHIR format),
and ORCA or Nuts are mapped to SCP entities.

| Mapped SCP Field                      | Source System | Source System Field                                                                  | Mapping                                                                               |
|---------------------------------------|---------------|--------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------|
| Launch context EHR BSN                | Zorgplatform  | `Patient.identifier (system=http://fhir.nl/fhir/NamingSystem/bsn)`                   | BSN from HiX auth token is ignored, taken from FHIR Patient resource instead          |
| Launch context EHR Task ID            | HiX           | Auth token (`http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id`) |                                                                                       |
| Launch context EHR Task               | Zorgplatform  |                                                                                      | Reference is `Task/<workflow ID>`, used to construct SCP Task                         |
| Launch context EHR Patient            | Zorgplatform  | Auth token (`http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id`) | Search narrowing guarantees that only the patient related to the workflow is returned |
| `Condition`                           | ORCA          |                                                                                      | Created by ORCA, as Zorgplatform doesn't provide a Condition.                         |
| `Condition.code`                      | Zorgplatform  | `Task.definitionReference`                                                           | ChipSoft workflow OID to snomed code (e.g. `urn:oid:2.16.840.1.113883.2.4.3.224.2.1`) |
| `ServiceRequest`                      | ORCA          |                                                                                      | Created by ORCA, as Zorgplatform doesn't provide a ServiceRequest.                    |
| `ServiceRequest.status`               | ORCA          |                                                                                      | `active` (hardcoded)                                                                  |
| `ServiceRequest.identifier`           | HiX           | Auth token (`http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id`) | `system=http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id`        |
| `ServiceRequest.code`                 | ORCA          |                                                                                      | `snomed\|719858009 telemonitoring` (hardcoded)                                        |
| `ServiceRequest.display`              | ORCA          |                                                                                      | `monitoren via telegeneeskunde` (hardcoded)                                           |
| `ServiceRequest.reasonReference`      | ORCA          |                                                                                      | Constructed Condition                                                                 |
| `ServiceRequest.subject.identifier`   | Zorgplatform  | `Patient.identifier (system=http://fhir.nl/fhir/NamingSystem/bsn)`                   | `plan` (hardcoded)                                                                    |
| `ServiceRequest.subject.reference`    | ORCA          | Launch context Patient reference                                                     |                                                                                       |
| `ServiceRequest.performer.identifier` | ORCA          |                                                                                      | Configured (`system=http://fhir.nl/fhir/NamingSystem/ura`)                            |
| `ServiceRequest.performer.display`    | Nuts          | `X509Credential.credentialSubject.subject.O`                                         | Read from CSD                                                                         |
| `ServiceRequest.requester.identifier` | Nuts          | `X509Credential.credentialSubject.subject.otherName`                                 | Read from local wallet (`system=http://fhir.nl/fhir/NamingSystem/ura`)                |
| `ServiceRequest.requester.display`    | Nuts          | `X509Credential.credentialSubject.subject.O`                                         | Read from local wallet                                                                |
| `Practitioner`                        | Zorgplatform  | `Patient.generalPractitioner`                                                        | Pre-populates QuestionnaireResponses                                                  |
| `Patient`                             | Zorgplatform  |                                                                                      | Sanitized Patient resource from Zorgplatform (see rows below for removed fields)      |
|                                       | Zorgplatform  | `Patient.contact.organization.reference`                                             | External literal reference (of Zorgplatform) removed                                  |
|                                       | Zorgplatform  | `Patient.managingOrganization.reference`                                             | External literal reference (of Zorgplatform) removed                                  |
|                                       | Zorgplatform  | `Patient.link.other.reference`                                                       | External literal reference (of Zorgplatform) removed                                  |
|                                       | Zorgplatform  | `Patient.generalPractitioner.reference`                                              | External literal reference (of Zorgplatform) removed                                  |
| `Task.meta.profile`                   | ORCA          |                                                                                      | `http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask`         |
| `Task.identifier`                     | Zorgplatform  | Auth token (`http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id`) |                                                                                       |
| `Task.for.type`                       | ORCA          |                                                                                      | `Patient` (hardcoded)                                                                 |
| `Task.for.reference`                  | ORCA          | Launch context Patient reference                                                     |                                                                                       |
| `Task.status`                         | ORCA          |                                                                                      | `requested` (hardcoded)                                                               |
| `Task.intent`                         | ORCA          |                                                                                      | `order` (hardcoded)                                                                   |
| `Task.intent`                         | ORCA          |                                                                                      | `order` (hardcoded)                                                                   |
| `Task.reasonCode`                     | ORCA          | `Condition.code`                                                                     |                                                                                       |
| `Task.reasonReference.reference`      | ORCA          | Launch context Condition reference                                                   | Constructed Condition                                                                 |
| `Task.reasonReference.display`        | ORCA          |                                                                                      | Depends on `Condition.code` (hardcoded)                                               |
| `Task.requester`                      | ORCA          | `ServiceRequest.requester.identifier`                                                |                                                                                       |
| `Task.performer`                      | ORCA          | `ServiceRequest.performer[0].identifier`                                             |                                                                                       |
| `Task.focus.type`                     | ORCA          |                                                                                      | `ServiceRequest` (hardcoded)                                                          |
| `Task.focus.reference`                | ORCA          | Launch context ServiceRequest reference                                              | Constructed ServiceRequest                                                            |
| `Task.focus.display`                  | ORCA          | `ServiceRequest.code.coding.[0].display`                                             |                                                                                       |

## Access via local deployment

1. `az login`
2. configure the kv values, e.g:
   ```json 
   {
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_ENABLED": "true",
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_SIGN_ISS": "<iss>", //The hl7 oid of the care organization configured in Zorgplatform
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_SIGN_AUD": "<aud>", //The STS URL
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_DECRYPT_ISS": "<iss>", //The STS URL
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_DECRYPT_AUD": "<aud>", //The service URL configured in Zorgplatform
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_DECRYPT_SIGNCERT": "<pem_certificate>", // A PEM-formatted X.509 certificate used to verify signatures provided by Zorgplatform. Should retain newlines, e.g. "-----BEGIN CERTIFICATE-----\nMIIGpTC...SIuTjA==\n-----END CERTIFICATE-----",
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_BASEURL": "<url>", //https://zorgplatform.online OR https://acceptatie.zorgplatform.online
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_STSURL": "<url>", //https://zorgplatform.online/sts OR https://acceptatie.zorgplatform.online/sts
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_APIURL": "<url>", //https://api.zorgplatform.online/fhir/V1/ OR https://api.acceptatie.zorgplatform.online/fhir/V1/
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_URL": "<url>", //The URL of the Azure KeyVault to use
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_CREDENTIALTYPE": "<type>", //The Azure credential type, "default", "cli" or "managed_identity"
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_DECRYPTCERTNAME": "<certname>", //Name of the KV decrypt certificate (used to decrypt assertions that are received from Zorgplatform)
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_SIGNCERTNAME": "<certname>", //Name of the KV signing certificate (used to sign assertions that wil be sent to Zorgplatform)
   "ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_ZORGPLATFORM_AZURE_KEYVAULT_CLIENTCERTNAME": "<certname>" //Name of the KV client certificate (used to set up mTLS with Zorgplatform)
   }
   ```