#!/bin/bash

set -e



# Create a root did:web DID. It takes the party name, Nuts node URL and Nuts internal API.
# It then converts the Nuts node URL to a rooted did:web DID, e.g. https://something -> did:web:something
# It then creates the DID at the Nuts node and returns the DID.
function createDID() {
  NAME=$1
  NUTS_INTERNAL_API=$2
  DIDDOC=$(curl -s -X POST -H "Content-Type:application/json" -d "{\"subject\": \"${NAME}\"}" $NUTS_INTERNAL_API/internal/vdr/v2/subject)
  DID=$(echo $DIDDOC | jq -r .documents[0].id)
  # If DID is null, it indicates an error in the creation of the DID. See if the subject already exists.
  if [ "$DID" == "null" ]; then
    RESPONSE=$(curl -s $NUTS_INTERNAL_API/internal/vdr/v2/subject/${NAME})
    DID=$(echo $RESPONSE | jq -r .[0])
  fi
  echo $DID
}

# Self-issue a NutsUraCredential. It takes:
# - Subject ID holding the wallet the VC should be loaded into
# - The DID of the entity to issue the credential to
# - The URA number of the entity
# - The name of the entity
# - The city of the entity
function issueUraCredential() {
  SUBJECT=$1
  DID=$2
  URA=$3
  NAME=$4
  CITY=$5

  REQUEST=$(
  cat << EOF
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://nuts.nl/credentials/2024"
  ],
  "type": "NutsUraCredential",
  "issuer": "${DID}",
  "credentialSubject": {
    "id": "${DID}",
    "organization": {
      "ura": "${URA}",
      "name": "${NAME}",
      "city": "${CITY}"
    }
  },
  "withStatusList2021Revocation": false
}
EOF
  )

   # Issue VC, read it from the response, load it into own wallet.
   RESPONSE=$(curl -s -X POST -d "$REQUEST" -H "Content-Type: application/json" http://nutsnode:8081/internal/vcr/v2/issuer/vc)
   curl -s -X POST -d "$RESPONSE" -H "Content-Type: application/json" "http://nutsnode:8081/internal/vcr/v2/holder/${SUBJECT}/vc"
}

echo "Creating stack for Hospital..."
export HOSPITAL_URL=http://localhost:8090
echo "  Creating DID document"
HOSPITAL_DID=$(createDID "hospital" http://nutsnode:8081)
echo "    Hospital DID: $HOSPITAL_DID"
echo "  Self-issuing an NutsUraCredential"
issueUraCredential "hospital" "${HOSPITAL_DID}" "4567" "Demo Hospital" "Amsterdam"
echo "  Registering on Nuts Discovery Service"
curl -s -X POST -H "Content-Type: application/json" -d "{\"registrationParameters\":{\"fhirBaseURL\": \"${HOSPITAL_URL}/cpc/hospital/fhir\", \"fhirNotificationURL\": \"${HOSPITAL_URL}/cpc/hospital/fhir\"}}" http://nutsnode:8081/internal/discovery/v1/dev:HomeMonitoring2024/hospital

echo "Creating stack for Clinic..."
export CLINIC_URL=http://localhost:8091
echo "  Creating DID document"
CLINIC_DID=$(createDID "clinic" http://nutsnode:8081)
echo "    Clinic DID: $CLINIC_DID"
echo "  Self-issuing an NutsUraCredential"
issueUraCredential "clinic" "${CLINIC_DID}" "1234" "Demo Clinic" "Utrecht"
echo "  Registering on Nuts Discovery Service"
curl -s -X POST -H "Content-Type: application/json" -d "{\"registrationParameters\":{\"fhirBaseURL\": \"${CLINIC_URL}/cpc/clinic/fhir\", \"fhirNotificationURL\": \"${CLINIC_URL}/cpc/clinic/fhir\"}}" http://nutsnode:8081/internal/discovery/v1/dev:HomeMonitoring2024/clinic

