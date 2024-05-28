@description('Specifies the location for all resources.')
param location string = resourceGroup().location

param uaiName string = 'uai-${uniqueString(resourceGroup().id)}'
param uaiResourceGroup string = resourceGroup().name
param uaiSubscription string = subscription().subscriptionId

param containerEnvName string = 'env-${uniqueString(resourceGroup().id)}'
param containerEnvResourceGroup string = resourceGroup().name
param containerEnvSubscription string = subscription().subscriptionId

param keyVaultName string = 'kev-${uniqueString(resourceGroup().id)}'

@minLength(3)
param healthWorkspaceName string = 'hdw${uniqueString(resourceGroup().id)}'
param serviceName string = 'orca'
param healthWorkspaceResourceGroup string = resourceGroup().name
param healthWorkspaceSubscription string = subscription().subscriptionId

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
  'debug'
  'info'
  'warn'
  'error'
])
param logLevel string = 'info'

param createNutsNode bool = true
param createOrchestrator bool = true
param createSoFbackend bool = false
= if (createLogAnalytics)

module nutsnode './modules/logAnalytics-new.bicep' = if (createNutsNode) {
  name: 'DeployNutsNode'
  scope: resourceGroup(containerEnvSubscription, containerEnvResourceGroup)
  params: {
    resourceName: 'nutsnode'
    location: location
  }
}

resource containerApp 'Microsoft.App/containerApps@2024-03-01' = {
  name: serviceName
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
              name: 'FHIRSERVER_HOST'
              value: fhirService.properties.authenticationConfiguration.audience
            }
            {
              name: 'HTTP_PORT'
              value: string(8080)
            }
            {
              name: 'TZ'
              value: 'Europe/Amsterdam'
            }
            {
              name: 'LOG_LEVEL'
              value: logLevel
            }
          ]
          // volumeMounts: [
          //   {
          //     mountPath: 'dockervolume'
          //     volumeName: '/mnt'
          //   }
          // ]
          name: 'orca-smartonfhir-backend-adapter'
          probes: [
            {
              failureThreshold: 3
              httpGet: {
                path: '/health'
                port: 5080
                scheme: 'HTTP'
              }
              initialDelaySeconds: 30
              timeoutSeconds: 5
              periodSeconds: 10
              successThreshold: 1
            }
          ]
          image: 'ghcr.io/santeonnl/orca_smartonfhir_backend_adapter:main'
          resources: {
            #disable-next-line BCP036
            cpu: json(cpuCore)
            memory: '${memorySize}Gi'
          }
        }
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
              name: 'FHIRSERVER_HOST'
              value: fhirService.properties.authenticationConfiguration.audience
            }
            {
              name: 'HTTP_PORT'
              value: string(8080)
            }
            {
              name: 'TZ'
              value: 'Europe/Amsterdam'
            }
            {
              name: 'LOG_LEVEL'
              value: logLevel
            }
          ]
          // volumeMounts: [
          //   {
          //     mountPath: 'dockervolume'
          //     volumeName: '/mnt'
          //   }
          // ]
          name: 'orca-orchestrator'
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
          image: 'ghcr.io/santeonnl/orca_orchestrator:main'
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
