@description('Specifies the location for all resources.')
param location string

// var rgNameArray = split(resourceGroup().name, '-')
// var deployenv = rgNameArray[4]

@description('Specifies the name of the container app environment.')
param resourceName string

@description('Specifies the name of the log analytics workspace.')
param logAnalyticsName string
param logAnalyticsSubscription string
param logAnalyticsResourceGroup string

param shareName string
param storageAccountName string
param storageAccountResourceGroup string
param storageAccountSubscription string

// @description('Do you want to restrict access to resource to the private network? (default = true, use "false" only for development deployenv)')
// var privatenetworkaccess = (deployenv != 'o')

// @description('Provide the name of the container app environment vNet')
// param vnetName string

// @description('Subnet Datahub Name')
// param subnetName string

resource storageAccount 'Microsoft.Storage/storageAccounts@2022-09-01' existing = {
  name: storageAccountName
  scope: resourceGroup(storageAccountSubscription, storageAccountResourceGroup)
}

resource fileShare 'Microsoft.Storage/storageAccounts/fileServices/shares@2022-09-01' existing = {
  name: '${storageAccountName}/default/${shareName}'
  scope: resourceGroup(storageAccountSubscription, storageAccountResourceGroup)
}

resource law 'Microsoft.OperationalInsights/workspaces@2022-10-01' existing = {
  name: logAnalyticsName
  scope: resourceGroup(logAnalyticsSubscription, logAnalyticsResourceGroup)
}

resource environment 'Microsoft.App/managedEnvironments@2024-03-01' = {
  name: resourceName
  location: location
  properties: {
    appLogsConfiguration: {
      destination: 'log-analytics'
      logAnalyticsConfiguration: {
        customerId: law.properties.customerId
        sharedKey: law.listKeys().primarySharedKey
      }
    }
    // vnetConfiguration: {
    //   internal: privatenetworkaccess
    //   infrastructureSubnetId: resourceId('Microsoft.Network/virtualNetworks/subnets', vnetName, subnetName)
    // }
  }
}

resource envstorage 'Microsoft.App/managedEnvironments/storages@2024-03-01' = {
  parent: environment
  name: 'mnt'
  properties: {
    azureFile: {
      accountName: storageAccountName
      shareName: shareName
      accountKey: storageAccount.listKeys().keys[0].value
      accessMode: 'ReadWrite'
    }
  }
}

// resource containerDiagnostics 'Microsoft.OperationalInsights/workspaces/providers/diagnosticSettings@2021-05-01' = {
//   name: '${environment.name}/Microsoft.Insights/ISO 27001 Compliance'
//   properties: {
//     workspaceId: law.id
//     logs: [
//       {
//         category: 'Audit'
//         enabled: true
//         retentionPolicy: {
//           enabled: true
//           days: 0
//         }
//       },
//       {
//         category: 'allLogs'
//         enabled: true
//         retentionPolicy: {
//           enabled: true
//           days: 0
//         }
//       }
//     ]
//     metrics: [
//       {
//         category: 'AllMetrics'
//         enabled: true
//         retentionPolicy: {
//           enabled: true
//           days: 0
//         }
//       }
//     ]
//   }
// }
