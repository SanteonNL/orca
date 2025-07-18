param resourceName string
param roleDefinitionName string
param roleDefinitionId string
param principalId string
param principalName string
param principalType string

resource sqlserver 'Microsoft.Sql/servers@2023-05-01-preview' existing = {
  name: resourceName
}

resource roleAssignment 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  scope: sqlserver
  name: guid(principalName, roleDefinitionName, sqlserver.name)
  properties: {
    description: '${principalName}_${roleDefinitionName}_${sqlserver.name}'
    roleDefinitionId: resourceId('Microsoft.Authorization/roleDefinitions', roleDefinitionId)
    principalId: principalId
    principalType: principalType
  }
}
