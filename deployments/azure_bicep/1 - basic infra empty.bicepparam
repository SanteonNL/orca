using './1 - basic infra.bicep'

param adminGroupObjectId = ''
param adminGroupObjectName = ''
param createUai = true
param createContainerEnv = true
param createFhirService = true
param createKeyVault = true
param createLogAnalytics = true
param createSqlserverDB = true
param createStorageAccount = true

/*
// If you would like to create the resources with a different name/resourceGroup/subscription, use these parameters. 
// You can also refer to existing resources by setting e.g. param createKeyVault = false and providing an existing keyVaultName, keyVaultResourceGroup and keyVaultSubscription
param uaiName = 
param uaiResourceGroup = 
param uaiSubscription = 

param logAnalyticsName = 
param logAnalyticsResourceGroup = 
param logAnalyticsSubscription = 

param keyVaultName = 
param keyVaultResourceGroup = 
param keyVaultSubscription = 

param storageAccountName = 
param storageAccountResourceGroup = 
param storageAccountSubscription = 

param sqlserverDBName = 
param sqlserverDBResourceGroup = 
param sqlserverDBSubscription = 

param containerEnvName = 
param containerEnvResourceGroup = 
param containerEnvSubscription = 

param fhirServiceName = 
param fhirServiceResourceGroup = 
param fhirServiceSubscription = 
*/
