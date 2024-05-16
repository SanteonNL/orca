# Azure Bicep Deployment
This Orca deployment targets Azure through Bicep.

It deploys the following resources:

- Nuts Node is deployed as Azure Container Instance, as it does not support clustering yet
- SQL database, Postgres, is deployed as Azure SQL Database

## Deploying
This deployment can be done using the Azure CLI (https://learn.microsoft.com/en-us/azure/azure-resource-manager/bicep/deploy-cli):

