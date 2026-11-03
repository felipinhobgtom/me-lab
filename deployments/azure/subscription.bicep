targetScope = 'subscription'

@description('Resource Group that owns all ME Lab resources.')
param resourceGroupName string = 'rg-melab-prod'

@description('Azure region for the Resource Group and ME Lab resources.')
param location string = 'brazilsouth'

@description('Short environment identifier.')
param environmentName string = 'prod'

@description('Azure VM size.')
param vmSize string = 'Standard_B2s'

@description('Administrator Linux username.')
param adminUsername string = 'azureuser'

@description('SSH public key for the VM administrator.')
@secure()
param sshPublicKey string

@description('Restricted CIDR allowed to access SSH.')
param sshAllowedCidr string

@description('Globally unique Azure DNS label prefix.')
param dnsLabelPrefix string = 'melab-${uniqueString(subscription().id)}'

@description('Immutable GHCR image tag.')
param appImageTag string = 'latest'

@description('GitHub repository used for GHCR image names.')
param githubRepository string = 'felipinhobgtom/me-lab'

@description('GHCR username used by the VM pull extension.')
param ghcrUsername string = 'github-actions'

@description('Read-only GHCR token used by the VM pull extension.')
@secure()
param ghcrPullToken string

@description('Application JWT signing secret.')
@secure()
param jwtSecret string

@description('Allowed browser origin for the API.')
param corsAllowedOrigins string = 'https://${dnsLabelPrefix}.${location}.cloudapp.azure.com'

resource resourceGroup 'Microsoft.Resources/resourceGroups@2024-03-01' = {
  name: resourceGroupName
  location: location
  tags: {
    application: 'me-lab'
    environment: environmentName
    managedBy: 'arm'
  }
}

module application 'main.bicep' = {
  name: 'me-lab-${environmentName}-${uniqueString(resourceGroup.id)}'
  scope: resourceGroup
  params: {
    location: location
    environmentName: environmentName
    vmSize: vmSize
    adminUsername: adminUsername
    sshPublicKey: sshPublicKey
    sshAllowedCidr: sshAllowedCidr
    dnsLabelPrefix: dnsLabelPrefix
    appImageTag: appImageTag
    githubRepository: githubRepository
    ghcrUsername: ghcrUsername
    ghcrPullToken: ghcrPullToken
    jwtSecret: jwtSecret
    corsAllowedOrigins: corsAllowedOrigins
  }
}

output resourceGroupName string = resourceGroup.name
output vmName string = application.outputs.vmName
output fqdn string = application.outputs.fqdn
output storageAccountName string = application.outputs.storageAccountName
