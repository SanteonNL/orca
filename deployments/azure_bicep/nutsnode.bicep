@description('Name for the container group')
param name string = 'orca-group'

@description('Location for all resources.')
param location string = resourceGroup().location

@description('The behavior of Azure runtime if container has stopped.')
@allowed([
    'Always'
    'Never'
    'OnFailure'
])
param restartPolicy string = 'Always'

// Nuts node container parameters
@description('Name for the Nuts Node container')
param nodeContainerName string = 'nutsnode'

@description('Nuts Node Docker image to deploy')
param nodeImage string = 'nutsfoundation/nuts-node:master'

// @description('Port to open on the container and the public IP address.')
// param port int = 80

@description('The number of CPU cores to allocate to the container.')
param nodeCPUCores int = 1

@description('The amount of memory to allocate to the container in gigabytes.')
param nodeMemoryInGB int = 2

resource containerGroup 'Microsoft.ContainerInstance/containerGroups@2023-05-01' = {
    name: name
    location: location
    properties: {
        containers: [
            {
                name: nodeContainerName
                properties: {
                    environmentVariables: [
                        {
                            name: 'NUTS_URL'
                            value: 'https://example.com'
                        }
                        {
                            name: 'NUTS_CRYPTO_STORAGE'
                            value: 'fs'
                        }
                    ]
                    image: nodeImage
                    //           ports: [
                    //             {
                    //               port: port
                    //               protocol: 'TCP'
                    //             }
                    //           ]
                    resources: {
                        requests: {
                            cpu: nodeCPUCores
                            memoryInGB: nodeMemoryInGB
                        }
                    }
                }
            }
        ]
        osType: 'Linux'
        restartPolicy: restartPolicy
        //     ipAddress: {
        //       type: 'Public'
        //       ports: [
        //         {
        //           port: port
        //           protocol: 'TCP'
        //         }
        //       ]
        //     }
    }
}

output name string = containerGroup.name
output resourceGroupName string = resourceGroup().name
output resourceId string = containerGroup.id
// TODO: does not work?
// output containerIPv4Address string = containerGroup.properties.ipAddress.ip
output location string = location