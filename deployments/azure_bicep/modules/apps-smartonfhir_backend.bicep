@description('Specifies the location for all resources.')
param location string = resourceGroup().location

@description('Name for the resource')
param resourceName string = 'smartonfhir-backend'

@description('Nuts Node Docker image to deploy')
param nodeImage string = 'ghcr.io/santeonnl/orca_smartonfhir_backend_adapter:main'

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
param cpuCore string = '0.5'

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
param memorySize string = '1'

@description('The level on which of app should log. Default loglevel: "info", Set to "debug" for more verbose logging')
@allowed([
  'trace'
  'debug'
  'info'
  'warn'
  'error'
])
param logLevel string = 'info'

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
        targetPort: 8080
        transport: 'auto'
        traffic: [
          {
            weight: 100
            latestRevision: true
          }
        ]
        // additionalPortMappings: [
        //   {
        //     external: true
        //     // For management -- We probably don't want to expose this in production, instead we should
        //     // serve the management UI from a sidecar, and only expose the sidecar.
        //     targetPort: 8081
        //   }
        // ]
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
      // initContainers: [
      //   {
      //     name: initContainer.name
      //     image: initContainer.image
      //     resources: {
      //       #disable-next-line BCP036
      //       cpu: initContainer.cpu
      //       memory: initContainer.memory
      //     }
      //     volumeMounts: nutsVolumes
      //   }
      // ]
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
              name: 'KEYVAULT'
              value: keyVaultName
            }
            {
              name: 'NUTS_URL'
              value: 'https://orca-test.com'
            }
            {
              name: 'NUTS_CRYPTO_STORAGE'
              value: 'fs'
            }
            {
              name: 'NUTS_AUTH_CONTRACTVALIDATORS'
              value: 'dummy'
            }
            {
              name: 'NUTS_HTTP_INTERNAL_ADDRESS'
              value: ':8081'
            }
            {
              name: 'FHIRSERVER_HOST'
              value: fhirService.properties.authenticationConfiguration.audience
            }
            {
              name: 'TZ'
              value: 'Europe/Amsterdam'
            }
            {
              name: 'NUTS_VERBOSITY'
              value: logLevel
            }
            {
              name: 'NUTS_LOGGERFORMAT'
              value: 'json'
            }
          ]
          volumeMounts: [
            {
              mountPath: '/mnt/${resourceName}-config'
              volumeName: 'dockervolume'
            }
          ]
          name: '${resourceName}-container'
          probes: [
            {
              failureThreshold: 3
              httpGet: {
                path: '/status'
                port: 8081
                scheme: 'HTTP'
              }
              initialDelaySeconds: 30
              timeoutSeconds: 5
              periodSeconds: 10
              successThreshold: 1
            }
          ]
          image: nodeImage
          resources: {
            #disable-next-line BCP036
            cpu: json(cpuCore)
            memory: '${memorySize}Gi'
          }
        }
      ]
      // Nuts does not scale. Ebin.
      scale: {
        minReplicas: 1
        maxReplicas: 1
      }
      volumes: [
        {
          name: 'dockervolume'
          storageType: 'AzureFile'
          // Each nuts environment has its own file share configured in the container app
          // environment level.
          storageName: 'mnt'
        }
      ]
    }
  }
}
