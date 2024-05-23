param resourceName string

resource uai 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' existing = {
  name: resourceName
}

output id string = uai.id
output clientId string = uai.properties.clientId
output principalId string = uai.properties.principalId
