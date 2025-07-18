@description('Specifies the location for all resources.')
param location string = resourceGroup().location

@description('Do you want to restrict access to resource to the private network? (default = true, use "false" only for development deployenv)')
var privatenetworkaccess = false

@description('storageAccount Name')
param resourceName string

@description('storageAccount share name (leave empty for no share)')
param shareName string

// @description('vnet Name')
// param vnetName string

// @description('Subnet Health data workspace Name')
// param subnetName string

resource storageacc 'Microsoft.Storage/storageAccounts@2022-09-01' = {
  sku: {
    name: 'Standard_ZRS'
  }
  kind: 'StorageV2'
  name: resourceName
  location: location
  properties: privatenetworkaccess
    ? {
        dnsEndpointType: 'Standard'
        defaultToOAuthAuthentication: false
        publicNetworkAccess: 'Enabled'
        allowCrossTenantReplication: false
        isSftpEnabled: false
        minimumTlsVersion: 'TLS1_2'
        allowBlobPublicAccess: true
        allowSharedKeyAccess: true
        isHnsEnabled: true
        networkAcls: {
          resourceAccessRules: []
          bypass: 'Logging, Metrics, AzureServices'
          // virtualNetworkRules: [
          //   {
          //     id: resourceId('Microsoft.Network/virtualNetworks/subnets', vnetName, subnetName)
          //     action: 'Allow'
          //   }
          // ]
          ipRules: []
          defaultAction: 'Deny'
        }
        supportsHttpsTrafficOnly: true
        encryption: {
          requireInfrastructureEncryption: true
          services: {
            file: {
              keyType: 'Account'
              enabled: true
            }
            blob: {
              keyType: 'Account'
              enabled: true
            }
          }
          keySource: 'Microsoft.Storage'
        }
        accessTier: 'Hot'
      }
    : {}
}

resource fileShare 'Microsoft.Storage/storageAccounts/fileServices/shares@2022-09-01' = if (!empty(shareName)) {
  name: '${storageacc.name}/default/${shareName}'
}
