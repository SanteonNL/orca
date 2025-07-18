@description('Specifies the location for all resources.')
param location string = resourceGroup().location

param resourceName string

resource newUai 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' = {
  name: resourceName
  location: location
}
