param resourceName string
param roleDefinitionName string
param roleDefinitionId string
param principalId string
param principalName string
param principalType string

resource fhirService 'Microsoft.HealthcareApis/workspaces/fhirservices@2022-12-01' existing = {
  name: resourceName
}

resource roleAssignment 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  scope: fhirService
  name: guid(principalName, roleDefinitionName, fhirService.name)
  properties: {
    description: '${principalName}_${roleDefinitionName}_${fhirService.name}'
    roleDefinitionId: resourceId('Microsoft.Authorization/roleDefinitions', roleDefinitionId)
    principalId: principalId
    principalType: principalType
  }
}

output audience string = fhirService.properties.authenticationConfiguration.audience
