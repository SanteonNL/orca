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
param createSmartOnFhirBackend bool = false

module nutsnode './modules/apps-nutsnode.bicep' = if (createNutsNode) {
  name: 'DeployNutsNode'
  scope: resourceGroup(containerEnvSubscription, containerEnvResourceGroup)
  params: {
    resourceName: 'nutsnode'
    uaiName: uaiName
    uaiResourceGroup: uaiResourceGroup
    uaiSubscription: uaiSubscription

    containerEnvName: containerEnvName

    keyVaultName: keyVaultName
    serviceName: serviceName
    healthWorkspaceName: healthWorkspaceName
    healthWorkspaceResourceGroup: healthWorkspaceResourceGroup
    healthWorkspaceSubscription: healthWorkspaceSubscription

    cpuCore: cpuCore
    memorySize: memorySize

    logLevel: logLevel
  }
}

module orchestrator './modules/apps-orchestrator.bicep' = if (createOrchestrator) {
  name: 'DeployOrchestrator'
  scope: resourceGroup(containerEnvSubscription, containerEnvResourceGroup)
  params: {
    resourceName: 'orchestrator'
    uaiName: uaiName
    uaiResourceGroup: uaiResourceGroup
    uaiSubscription: uaiSubscription

    containerEnvName: containerEnvName

    keyVaultName: keyVaultName
    serviceName: serviceName
    healthWorkspaceName: healthWorkspaceName
    healthWorkspaceResourceGroup: healthWorkspaceResourceGroup
    healthWorkspaceSubscription: healthWorkspaceSubscription

    cpuCore: cpuCore
    memorySize: memorySize

    logLevel: logLevel
  }
}

module smartonfhirbackend './modules/apps-nutsnode.bicep' = if (createSmartOnFhirBackend) {
  name: 'DeploySmartOnFhirBackend'
  scope: resourceGroup(containerEnvSubscription, containerEnvResourceGroup)
  params: {
    resourceName: 'smartonfhir-backend'
    uaiName: uaiName
    uaiResourceGroup: uaiResourceGroup
    uaiSubscription: uaiSubscription

    containerEnvName: containerEnvName

    keyVaultName: keyVaultName
    serviceName: serviceName
    healthWorkspaceName: healthWorkspaceName
    healthWorkspaceResourceGroup: healthWorkspaceResourceGroup
    healthWorkspaceSubscription: healthWorkspaceSubscription

    cpuCore: cpuCore
    memorySize: memorySize

    logLevel: logLevel
  }
}
