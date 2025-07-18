@description('Specifies the location for resources.')
param location string = resourceGroup().location

param healthWorkspaceName string
param fhirServiceName string

@description('Specifies the name of the log analytics workspace.')
param logAnalyticsName string
param logAnalyticsSubscription string
param logAnalyticsResourceGroup string

// @description('Provide the name of the container app environment vNet')
// param vnetName string

// @description('Subnet Health data workspace Name')
// param subnetName string

// @description('Do you want to restrict access to resource to the private network? (default = true, use "false" only for development deployenv)')
// param privatenetworkaccess bool = true

resource healthcareworkspace 'Microsoft.HealthcareApis/workspaces@2022-12-01' = {
  name: healthWorkspaceName
  location: location
  properties: {
    publicNetworkAccess: 'Enabled' //privatenetworkaccess ? 'Disabled' : 'Enabled'
  }
}

// var pepHDWserviceName = 'pep-${locationShort}-${company}-hdw-${deployenv}'
// resource pepHDWservice 'Microsoft.Network/privateEndpoints@2021-05-01' = if (deployenv != 'o') {
//   name: pepHDWserviceName
//   location: location
//   dependsOn: [fhirService]
//   properties: {
//     subnet: {
//       id: resourceId('Microsoft.Network/virtualNetworks/subnets', vnetName, subnetName)
//     }
//     privateLinkServiceConnections: [
//       {
//         name: pepHDWserviceName
//         properties: {
//           privateLinkServiceId: healthcareworkspace.id
//           groupIds: [
//             'healthcareworkspace'
//           ]
//         }
//       }
//     ]
//     customNetworkInterfaceName: 'pepnic-${locationShort}-${company}-hdw-${deployenv}'
//   }
// }

// @description('Generated from /subscriptions/873b843b-f2e3-49da-914b-b6fb50fd9c4c/resourceGroups/rg-we-zbj-datahub-t/providers/Microsoft.Network/privateEndpoints/pep-we-zbj-hdw-t')
// resource pepDnsZone 'Microsoft.Network/privateEndpoints/privateDnsZoneGroups@2022-07-01' = if (deployenv != 'o') {
//   name: 'default'
//   parent: pepHDWservice
//   properties: {
//     privateDnsZoneConfigs: [
//       {
//         name: 'pdzconfig-${locationShort}-${company}-hdw-${deployenv}'
//         properties: {
//           privateDnsZoneId: resourceId(
//             'a94a5f10-98d6-40e1-a9cc-2c018fedd79c',
//             'rg-we-san-hub-network-p',
//             'Microsoft.Network/privateDnsZones',
//             'azurehealthcareapis.com'
//           )
//         }
//       }
//     ]
//   }
// }

// resource logAnalyticsWorkspace 'Microsoft.OperationalInsights/workspaces@2022-10-01' existing = {
//   name: logAnalyticsWorkspaceName
// }

// resource pepnicDiagnostics 'Microsoft.OperationalInsights/workspaces/providers/diagnosticSettings@2021-05-01' = {
//   name: '${pepHDWservice.name}/Microsoft.Insights/ISO 27001 Compliance'
//   properties: {
//     workspaceId: logAnalyticsWorkspace.id
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

resource fhirService 'Microsoft.HealthcareApis/workspaces/fhirservices@2022-12-01' = {
  parent: healthcareworkspace
  name: fhirServiceName
  location: location
  kind: 'fhir-R4'
  identity: {
    type: 'SystemAssigned'
  }
  properties: {
    authenticationConfiguration: {
      audience: 'https://${healthWorkspaceName}-${fhirServiceName}.fhir.azurehealthcareapis.com'
      authority: uri(environment().authentication.loginEndpoint, subscription().tenantId)
      smartProxyEnabled: false
    }
    // acrConfiguration: {
    //   loginServers: [containerRegistry.properties.loginServer]
    // }
    // corsConfiguration: {
    //   origins: [
    //     '*'
    //   ]
    //   headers: [
    //     '*'
    //   ]
    //   methods: [
    //     'DELETE'
    //     'GET'
    //     'OPTIONS'
    //     'PATCH'
    //     'POST'
    //     'PUT'
    //   ]
    //   maxAge: 600
    //   allowCredentials: false
    // }
    resourceVersionPolicyConfiguration: {
      default: 'versioned'
    }
    publicNetworkAccess: 'Enabled' //privatenetworkaccess ? 'Disabled' : 'Enabled'
  }
}
output name string = fhirService.name

// var acrPullRole = resourceId('Microsoft.Authorization/roleDefinitions', '7f951dda-4ed3-4680-a7ca-43fe172d538d')

// @description('This allows the managed identity of the fhir-service-managed-identity to access the container registry')
// resource FhirServiceID_AcrPull_ContainerRegistry 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
//   scope: containerRegistry
//   name: guid(fhirService.id, acrPullRole, containerRegistry.id)
//   properties: {
//     description: 'FhirServiceID_AcrPull_ContainerRegistry'
//     roleDefinitionId: acrPullRole
//     principalId: fhirService.identity.principalId
//     principalType: 'ServicePrincipal'
//   }
// }

resource logAnalytics 'Microsoft.OperationalInsights/workspaces@2022-10-01' existing = {
  name: logAnalyticsName
  scope: resourceGroup(logAnalyticsSubscription, logAnalyticsResourceGroup)
}

resource fhirLogSettings 'Microsoft.Insights/diagnosticSettings@2021-05-01-preview' = {
  name: 'lawset-${uniqueString(resourceGroup().id)}'
  scope: fhirService
  properties: {
    workspaceId: logAnalytics.id
    metrics: [
      {
        category: 'AllMetrics'
        enabled: true
        // retentionPolicy: {
        //   enabled: true
        //   days: 30
        // }
      }
    ]
    logs: [
      {
        categoryGroup: 'allLogs'
        enabled: true
        // retentionPolicy: {
        //   enabled: true
        //   days: 30
        // }
      }
    ]
  }
}
