@description('Specifies the location for all resources.')
param location string = resourceGroup().location

param resourceName string
param dbName string

param adminGroupObjectId string
param adminGroupObjectName string

resource sqlserver 'Microsoft.Sql/servers@2023-05-01-preview' = {
  name: resourceName
  location: location
  identity: {
    type: 'SystemAssigned'
  }
  properties: {
    administrators: {
      administratorType: 'ActiveDirectory'
      login: adminGroupObjectName
      azureADOnlyAuthentication: false
      principalType: 'Group'
      sid: adminGroupObjectId
      tenantId: tenant().tenantId
    }
  }
}

resource symbolicname 'Microsoft.Sql/servers/databases@2023-05-01-preview' = {
  name: dbName
  location: location
  parent: sqlserver
  sku: {
    name: 'GP_S_Gen5'
    tier: 'GeneralPurpose'
    family: 'Gen5'
    capacity: 1
  }
  properties: {
    collation: 'SQL_Latin1_General_CP1_CI_AS'
  }
}
