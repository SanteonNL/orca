# Azure Bicep Deployment
This ORCA deployment targets Azure through Bicep.

It deploys the following resources:

- Nuts Node is deployed as Azure Container Instance, as it does not support clustering yet (so no need for scaling)
- (TODO) Orchestrator is deployed as Azure Container Instance/App Service(?) 
- (TODO) SMART on FIR Adapter is deployed as Azure Container Instance/App Service(?) 
- (TODO) SQL database, Postgres, is deployed as Azure SQL Database
- (TODO) Azure Key Vault is deployed for storing private keys

## Deploying
This deployment can be done using the Azure CLI (https://learn.microsoft.com/en-us/azure/azure-resource-manager/bicep/deploy-cli):

```shell
az deployment group create --resource-group <resource group> \
  --template-file nutsnode.bicep
```