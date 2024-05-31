@description('Specifies the location for all resources.')
param location string = resourceGroup().location

param resourceName string

resource keyVault 'Microsoft.KeyVault/vaults@2022-11-01' = {
  name: resourceName
  location: location
  properties: {
    enabledForDeployment: true
    enabledForTemplateDeployment: true
    enabledForDiskEncryption: true
    tenantId: subscription().tenantId
    sku: {
      name: 'standard'
      family: 'A'
    }
    enableRbacAuthorization: true
  }
}

// resource key 'Microsoft.KeyVault/vaults/keys@2023-07-01' = {
//   name: 'smart-on-fhir-backend-signing-key'
//   properties: {}
// }

// resource keyVaultDiagnostics 'Microsoft.OperationalInsights/workspaces/providers/diagnosticSettings@2021-05-01' = {
//   name: '${keyVault.name}/Microsoft.Insights/ISO 27001 Compliance'
//   properties: {
//     workspaceId: logAnalytics.id
//     logs: [
//       {
//         category: 'Audit'
//         enabled: true
//         retentionPolicy: {
//           enabled: true
//           days: 0
//         }
//       }
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
