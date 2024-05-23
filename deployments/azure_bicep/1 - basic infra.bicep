@description('Specifies the location for all resources.')
param location string = resourceGroup().location

@description('Specifies the AAD-group that should be able to READ and WRITE secrets')
param adminGroupObjectId string
param adminGroupObjectName string

param createUai bool
param uaiName string = 'uai-${uniqueString(resourceGroup().id)}'
param uaiResourceGroup string = resourceGroup().name
param uaiSubscription string = subscription().subscriptionId

param createLogAnalytics bool
param logAnalyticsName string = 'law-${uniqueString(resourceGroup().id)}'
param logAnalyticsResourceGroup string = resourceGroup().name
param logAnalyticsSubscription string = subscription().subscriptionId

param createKeyVault bool
param keyVaultName string = 'kev-${uniqueString(resourceGroup().id)}'
param keyVaultResourceGroup string = resourceGroup().name
param keyVaultSubscription string = subscription().subscriptionId

param createStorageAccount bool
param storageAccountName string = 'sa${uniqueString(resourceGroup().id)}'
param storageAccountResourceGroup string = resourceGroup().name
param storageAccountSubscription string = subscription().subscriptionId

param createSqlserverDB bool
param sqlserverDBName string = 'sds-${uniqueString(resourceGroup().id)}'
param sqlserverDBResourceGroup string = resourceGroup().name
param sqlserverDBSubscription string = subscription().subscriptionId

param createContainerEnv bool
param containerEnvName string = 'env-${uniqueString(resourceGroup().id)}'
param containerEnvResourceGroup string = resourceGroup().name
param containerEnvSubscription string = subscription().subscriptionId

param createFhirService bool
@minLength(3)
param healthWorkspaceName string = 'hdw${uniqueString(resourceGroup().id)}'
param serviceName string = 'orca'
param healthWorkspaceResourceGroup string = resourceGroup().name
param healthWorkspaceSubscription string = subscription().subscriptionId

module uai_new 'modules/uai-new.bicep' = if (createUai) {
  name: 'DeployUAInew'
  scope: resourceGroup(uaiSubscription, uaiResourceGroup)
  params: {
    resourceName: uaiName
    location: location
  }
}

module uai 'modules/uai.bicep' = {
  name: 'getUAIPrincipalId'
  dependsOn: [
    uai_new
  ]
  scope: resourceGroup(uaiSubscription, uaiResourceGroup)
  params: {
    resourceName: uaiName
  }
}

module logAnalytics './modules/logAnalytics-new.bicep' = if (createLogAnalytics) {
  name: 'DeployLogAnalytics'
  scope: resourceGroup(logAnalyticsSubscription, logAnalyticsResourceGroup)
  params: {
    resourcename: logAnalyticsName
    location: location
  }
}

module keyVault './modules/keyvault-new.bicep' = if (createKeyVault) {
  name: 'DeployKeyVault'
  scope: resourceGroup(keyVaultSubscription, keyVaultResourceGroup)
  dependsOn: [
    logAnalytics
  ]
  params: {
    resourceName: keyVaultName
    location: location
  }
}

module keyVaultAutzAdmin './modules/keyvault-authorization.bicep' = {
  name: 'DeployKeyVaultAutz'
  scope: resourceGroup(keyVaultSubscription, keyVaultResourceGroup)
  dependsOn: [
    keyVault
  ]
  params: {
    resourceName: keyVaultName
    roleDefinitionName: 'Secret Officer'
    roleDefinitionId: 'b86a8fe4-44ce-4948-aee5-eccb2c155cd7'
    principalName: adminGroupObjectName
    principalId: adminGroupObjectId
    principalType: 'Group'
  }
}

module keyVaultAutzUai './modules/keyvault-authorization.bicep' = {
  name: 'DeployKeyVaultUai'
  scope: resourceGroup(keyVaultSubscription, keyVaultResourceGroup)
  dependsOn: [
    keyVault
  ]
  params: {
    resourceName: keyVaultName
    roleDefinitionName: 'Secret User'
    roleDefinitionId: '4633458b-17de-408a-b874-0445c86b69e6'
    principalName: uaiName
    principalId: uai.outputs.principalId
    principalType: 'ServicePrincipal'
  }
}

module storageAccount './modules/storageAccount-new.bicep' = if (createStorageAccount) {
  name: 'DeployStorageAccount'
  scope: resourceGroup(storageAccountSubscription, storageAccountResourceGroup)
  dependsOn: [
    logAnalytics
  ]
  params: {
    location: location
    resourceName: storageAccountName
    shareName: serviceName
    // vnetName: vnetName
    // subnetName: subnetHDWName
  }
}

module sqlserverDB './modules/sqlserverdatabase-new.bicep' = if (createSqlserverDB) {
  name: 'DeploySqlserverDB'
  scope: resourceGroup(sqlserverDBSubscription, sqlserverDBResourceGroup)
  dependsOn: [
    logAnalytics
  ]
  params: {
    resourceName: sqlserverDBName
    dbName: serviceName
    location: location
    adminGroupObjectId: adminGroupObjectId
    adminGroupObjectName: adminGroupObjectName
  }
}

module containerEnv './modules/containerEnv-new.bicep' = if (createContainerEnv) {
  name: 'DeployContainerEnv'
  scope: resourceGroup(containerEnvSubscription, containerEnvResourceGroup)
  dependsOn: [
    logAnalytics
  ]
  params: {
    resourceName: containerEnvName
    location: location
    storageAccountName: storageAccountName
    shareName: serviceName
    storageAccountResourceGroup: storageAccountResourceGroup
    storageAccountSubscription: storageAccountSubscription
    logAnalyticsName: logAnalyticsName
    logAnalyticsSubscription: logAnalyticsSubscription
    logAnalyticsResourceGroup: logAnalyticsResourceGroup
  }
}

module healthWorkspace './modules/healthWorkspace-new.bicep' = if (createFhirService) {
  name: 'DeployFhirService'
  scope: resourceGroup(healthWorkspaceSubscription, healthWorkspaceResourceGroup)
  dependsOn: [
    logAnalytics
    storageAccount
  ]
  params: {
    location: location
    // vnetName: vnetName
    // subnetName: subnetHDWName
    healthWorkspaceName: healthWorkspaceName
    fhirServiceName: serviceName
    logAnalyticsName: logAnalyticsName
    logAnalyticsSubscription: logAnalyticsSubscription
    logAnalyticsResourceGroup: logAnalyticsResourceGroup
  }
}

module fhirServiceAutzUai './modules/fhirService-authorization.bicep' = {
  name: 'DeployfhirServiceAutzUai'
  scope: resourceGroup(healthWorkspaceSubscription, healthWorkspaceResourceGroup)
  dependsOn: [
    healthWorkspace
  ]
  params: {
    resourceName: '${healthWorkspaceName}/${serviceName}'
    roleDefinitionName: 'FHIR Contributor'
    roleDefinitionId: '5a1fc7df-4bf1-4951-a576-89034ee01acd'
    principalName: uaiName
    principalId: uai.outputs.principalId
    principalType: 'ServicePrincipal'
  }
}
