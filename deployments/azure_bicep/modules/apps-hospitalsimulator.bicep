@description('Specifies the location for all resources.')
param location string = resourceGroup().location

@description('Name for the resource')
param resourceName string = 'hospitalsimulator'

@description('Hospital Simulator Docker image to deploy')
param simulatorImage string = 'ghcr.io/santeonnl/orca_hospitalsimulator:main'

// @description('Port to open on the container and the public IP address.')
// param port int = 80

param uaiName string = 'uai-${uniqueString(resourceGroup().id)}'
param uaiResourceGroup string = resourceGroup().name
param uaiSubscription string = subscription().subscriptionId

param containerEnvName string = 'env-${uniqueString(resourceGroup().id)}'

param keyVaultName string = 'kev-${uniqueString(resourceGroup().id)}'

@minLength(3)
param healthWorkspaceName string = 'hdw${uniqueString(resourceGroup().id)}'

param serviceName string = 'orca'

param healthWorkspaceResourceGroup string = resourceGroup().name
param healthWorkspaceSubscription string = subscription().subscriptionId

resource uai 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' existing = {
  name: uaiName
  scope: resourceGroup(uaiSubscription, uaiResourceGroup)
}

resource environment 'Microsoft.App/managedEnvironments@2024-03-01' existing = {
  name: containerEnvName
}

resource fhirService 'Microsoft.HealthcareApis/workspaces/fhirservices@2022-12-01' existing = {
  name: '${healthWorkspaceName}/${serviceName}'
  scope: resourceGroup(healthWorkspaceSubscription, healthWorkspaceResourceGroup)
}

@description('Number of CPU cores the container can use. Can be with a maximum of two decimals.')
@allowed([
  '0.25'
  '0.5'
  '0.75'
  '1'
  '1.25'
  '1.5'
  '1.75'
  '2'
])
param cpuCore string = '0.25'

@description('Amount of memory (in gibibytes, GiB) allocated to the container up to 4GiB. Can be with a maximum of two decimals. Ratio with CPU cores must be equal to 2.')
@allowed([
  '0.5'
  '1'
  '1.5'
  '2'
  '3'
  '3.5'
  '4'
])
param memorySize string = '0.5'

resource containerApp 'Microsoft.App/containerApps@2024-03-01' = {
  name: resourceName
  location: location
  dependsOn: [uai, environment, fhirService]

  //tags: tagList
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: {
      '${uai.id}': {}
    }
  }
  properties: {
    managedEnvironmentId: environment.id
    configuration: {
      ingress: {
        external: true
        targetPort: 3000
        transport: 'auto'
        traffic: [
          {
            weight: 100
            latestRevision: true
          }
        ]
        allowInsecure: false
      }
      // registries: [
      //   {
      //     identity: containerIdentity.id
      //     server: containerRegistry.properties.loginServer
      //   }
      // ]
    }
    template: {
      containers: [
        {
          env: [
            {
              name: 'AZURE_TENANT_ID'
              value: subscription().tenantId
            }
            {
              name: 'AZURE_CLIENT_ID'
              value: uai.properties.clientId
            }
            {
              name: 'NEXT_PUBLIC_ORCA_BASE_URL'
              value: 'https://orchestrator.icystone-47561fa8.westeurope.azurecontainerapps.io' // TODO: make this configurable
            }
            {
                name: 'NEXT_PUBLIC_FHIR_BASE_URL'
                value: 'https://example.com'
            }
            {
                name: 'NEXT_PUBLIC_FHIR_BASE_URL_DOCKER'
                value: 'https://example.com'
            }
            {
              name: 'TZ'
              value: 'Europe/Amsterdam'
            }
          ]
          name: '${resourceName}-container'
          probes: [
            {
              failureThreshold: 3
              httpGet: {
                path: '/'
                port: 3000
                scheme: 'HTTP'
              }
              initialDelaySeconds: 30
              timeoutSeconds: 5
              periodSeconds: 10
              successThreshold: 1
            }
          ]
          image: simulatorImage
          resources: {
            #disable-next-line BCP036
            cpu: json(cpuCore)
            memory: '${memorySize}Gi'
          }
        }
      ]
      // Probably has in-memory state for now, so do not scale
      scale: {
        minReplicas: 1
        maxReplicas: 1
      }
    }
  }
}
