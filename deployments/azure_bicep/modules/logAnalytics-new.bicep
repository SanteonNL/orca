@description('Specifies the location for all resources.')
param location string = resourceGroup().location

param resourceName string

resource logAnalytics 'Microsoft.OperationalInsights/workspaces@2022-10-01' = {
  name: resourceName
  location: location
  properties: any({
    retentionInDays: 30
    features: {
      searchVersion: 1
    }
    sku: {
      name: 'PerGB2018'
    }
  })
}

// resource logAnalyticsDiagnostics 'Microsoft.OperationalInsights/workspaces/providers/diagnosticSettings@2021-05-01' = {
//   name: '${logAnalytics.name}/Microsoft.Insights/ISO 27001 Compliance'
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
//       },{
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
