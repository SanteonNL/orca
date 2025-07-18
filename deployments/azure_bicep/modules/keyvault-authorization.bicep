param resourceName string
param roleDefinitionName string
param roleDefinitionId string
param principalId string
param principalName string
param principalType string

resource keyVault 'Microsoft.KeyVault/vaults@2022-11-01' existing = {
  name: resourceName
}

resource roleAssignment 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  scope: keyVault
  name: guid(principalId, roleDefinitionId, keyVault.id)
  properties: {
    description: '${principalName}_${roleDefinitionName}_${keyVault.name}'
    roleDefinitionId: resourceId('Microsoft.Authorization/roleDefinitions', roleDefinitionId)
    principalId: principalId
    principalType: principalType
  }
}
