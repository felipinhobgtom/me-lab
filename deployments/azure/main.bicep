@description('Azure region for all ME Lab resources.')
param location string = resourceGroup().location

@description('Short environment identifier (e.g., prod, staging).')
@minLength(2)
@maxLength(12)
param environmentName string = 'prod'

@description('Azure VM size. Standard_D2as_v4 provides 2 vCPUs and 8 GiB for Go + Nginx + SQLite in Azure for Students regions.')
param vmSize string = 'Standard_D2as_v4'

@description('Administrator Linux username for SSH login.')
param adminUsername string = 'azureuser'

@description('SSH Ed25519 or RSA public key for the Linux administrator.')
@secure()
param sshPublicKey string

@description('Restricted CIDR block permitted to connect to SSH port 22 (e.g., 203.0.113.10/32). Never use 0.0.0.0/0 in production.')
param sshAllowedCidr string

@description('Globally unique DNS label prefix (<dnsLabelPrefix>.<region>.cloudapp.azure.com).')
param dnsLabelPrefix string = 'melab-${uniqueString(resourceGroup().id)}'

@description('Size in GiB of the persistent Standard SSD Managed Data Disk mounted at /mnt/melab-data.')
@minValue(4)
@maxValue(1024)
param dataDiskSizeGiB int = 32

@description('Immutable GHCR image tag deployed by the ARM VM extension.')
@minLength(1)
param appImageTag string = 'latest'

@description('GitHub repository used to resolve the GHCR image names.')
param githubRepository string = 'felipinhobgtom/me-lab'

@description('GHCR username used by the VM to pull private images.')
param ghcrUsername string = 'github-actions'

@description('Read-only GHCR token for the VM image pull.')
@secure()
param ghcrPullToken string

@description('Application JWT signing secret. Preserved in the VM .env after first deployment.')
@secure()
param jwtSecret string

@description('Allowed browser origins for the API.')
param corsAllowedOrigins string = 'https://${dnsLabelPrefix}.${location}.cloudapp.azure.com'

var nameSuffix = '${environmentName}-${take(uniqueString(resourceGroup().id), 6)}'
var vmName = 'vm-melab-${environmentName}'
var nsgName = 'nsg-melab-${nameSuffix}'
var vnetName = 'vnet-melab-${nameSuffix}'
var subnetName = 'snet-melab-app'
var publicIpName = 'pip-melab-${nameSuffix}'
var nicName = 'nic-melab-${nameSuffix}'
var dataDiskName = 'disk-melab-sqlite-${nameSuffix}'
var storageAccountName = take(toLower('melab${uniqueString(resourceGroup().id, environmentName)}'), 24)
var backupContainerName = 'sqlite-backups'

var composeFileBase64 = base64(loadTextContent('compose.prod.yaml'))
var appDeployCommand = format('''
/bin/bash -c "
set -euo pipefail
for attempt in $(seq 1 90); do
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    break
  fi
  sleep 10
done
command -v docker
mkdir -p /opt/me-lab/deployments/azure
echo "{0}" | base64 -d > /opt/me-lab/deployments/azure/compose.prod.yaml
if [ ! -s /opt/me-lab/.env ]; then
  umask 077
  cat > /opt/me-lab/.env <<ENV
PORT=8080
DATABASE_URL=/mnt/melab-data/me-lab.db
JWT_SECRET={1}
JWT_EXPIRY_HOURS=24
LOG_LEVEL=info
CORS_ALLOWED_ORIGINS={2}
GITHUB_REPOSITORY={3}
AZURE_STORAGE_ACCOUNT={4}
AZURE_STORAGE_CONTAINER={5}
AZURE_USE_MANAGED_IDENTITY=true
RETENTION_DAYS=30
ENV
  chmod 0600 /opt/me-lab/.env
fi
echo "{6}" | docker login ghcr.io --username "{7}" --password-stdin
cd /opt/me-lab
IMAGE_TAG="{8}" GITHUB_REPOSITORY="{3}" docker compose -f deployments/azure/compose.prod.yaml pull
IMAGE_TAG="{8}" GITHUB_REPOSITORY="{3}" docker compose -f deployments/azure/compose.prod.yaml up -d --remove-orphans
docker logout ghcr.io >/dev/null 2>&1 || true
"
''', composeFileBase64, jwtSecret, corsAllowedOrigins, githubRepository, storageAccountName, backupContainerName, ghcrPullToken, ghcrUsername, appImageTag)

// Azure built-in role: Storage Blob Data Contributor
var storageBlobDataContributorRoleId = subscriptionResourceId(
  'Microsoft.Authorization/roleDefinitions',
  'ba92f5b4-2d11-453d-a403-e96b0029c9fe'
)

resource nsg 'Microsoft.Network/networkSecurityGroups@2023-11-01' = {
  name: nsgName
  location: location
  properties: {
    securityRules: [
      {
        name: 'Allow-HTTP-ACME'
        properties: {
          description: 'Allow inbound HTTP 80 for Let\'s Encrypt HTTP-01 challenge and HTTPS redirect.'
          protocol: 'Tcp'
          sourcePortRange: '*'
          destinationPortRange: '80'
          sourceAddressPrefix: 'Internet'
          destinationAddressPrefix: '*'
          access: 'Allow'
          priority: 100
          direction: 'Inbound'
        }
      }
      {
        name: 'Allow-HTTPS'
        properties: {
          description: 'Allow inbound HTTPS 443 to Host Nginx TLS reverse proxy.'
          protocol: 'Tcp'
          sourcePortRange: '*'
          destinationPortRange: '443'
          sourceAddressPrefix: 'Internet'
          destinationAddressPrefix: '*'
          access: 'Allow'
          priority: 110
          direction: 'Inbound'
        }
      }
      {
        name: 'Allow-SSH-Restricted'
        properties: {
          description: 'Allow inbound SSH 22 strictly from operator CIDR.'
          protocol: 'Tcp'
          sourcePortRange: '*'
          destinationPortRange: '22'
          sourceAddressPrefix: sshAllowedCidr
          destinationAddressPrefix: '*'
          access: 'Allow'
          priority: 120
          direction: 'Inbound'
        }
      }
    ]
  }
}

resource publicIp 'Microsoft.Network/publicIPAddresses@2023-11-01' = {
  name: publicIpName
  location: location
  sku: {
    name: 'Standard'
    tier: 'Regional'
  }
  properties: {
    publicIPAllocationMethod: 'Static'
    publicIPAddressVersion: 'IPv4'
    dnsSettings: {
      domainNameLabel: toLower(dnsLabelPrefix)
    }
  }
}

resource vnet 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: vnetName
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: [
        '10.20.0.0/16'
      ]
    }
    subnets: [
      {
        name: subnetName
        properties: {
          addressPrefix: '10.20.1.0/24'
          networkSecurityGroup: {
            id: nsg.id
          }
        }
      }
    ]
  }
}

resource nic 'Microsoft.Network/networkInterfaces@2023-11-01' = {
  name: nicName
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          privateIPAllocationMethod: 'Dynamic'
          publicIPAddress: {
            id: publicIp.id
          }
          subnet: {
            id: '${vnet.id}/subnets/${subnetName}'
          }
        }
      }
    ]
  }
}

resource dataDisk 'Microsoft.Compute/disks@2023-10-02' = {
  name: dataDiskName
  location: location
  sku: {
    name: 'StandardSSD_LRS'
  }
  properties: {
    diskSizeGB: dataDiskSizeGiB
    creationData: {
      createOption: 'Empty'
    }
  }
}

resource vm 'Microsoft.Compute/virtualMachines@2024-03-01' = {
  name: vmName
  location: location
  identity: {
    type: 'SystemAssigned'
  }
  properties: {
    hardwareProfile: {
      vmSize: vmSize
    }
    osProfile: {
      computerName: vmName
      adminUsername: adminUsername
      customData: base64(loadTextContent('cloud-init.yaml'))
      linuxConfiguration: {
        disablePasswordAuthentication: true
        ssh: {
          publicKeys: [
            {
              path: '/home/${adminUsername}/.ssh/authorized_keys'
              keyData: sshPublicKey
            }
          ]
        }
      }
    }
    storageProfile: {
      imageReference: {
        publisher: 'Canonical'
        offer: 'ubuntu-24_04-lts'
        sku: 'server'
        version: 'latest'
      }
      osDisk: {
        createOption: 'FromImage'
        diskSizeGB: 30
        deleteOption: 'Delete'
        managedDisk: {
          storageAccountType: 'StandardSSD_LRS'
        }
      }
      dataDisks: [
        {
          lun: 0
          name: dataDisk.name
          createOption: 'Attach'
          caching: 'None'
          deleteOption: 'Detach'
          managedDisk: {
            id: dataDisk.id
          }
        }
      ]
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: nic.id
          properties: {
            deleteOption: 'Delete'
          }
        }
      ]
    }
  }
}

resource appDeploy 'Microsoft.Compute/virtualMachines/extensions@2024-03-01' = {
  name: 'app-deploy'
  parent: vm
  location: location
  properties: {
    publisher: 'Microsoft.Azure.Extensions'
    type: 'CustomScript'
    typeHandlerVersion: '2.1'
    autoUpgradeMinorVersion: true
    forceUpdateTag: appImageTag
    settings: {}
    protectedSettings: {
      commandToExecute: appDeployCommand
    }
  }
}

resource storageAccount 'Microsoft.Storage/storageAccounts@2023-05-01' = {
  name: storageAccountName
  location: location
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    accessTier: 'Hot'
    allowBlobPublicAccess: false
    minimumTlsVersion: 'TLS1_2'
    supportsHttpsTrafficOnly: true
  }
}

resource blobService 'Microsoft.Storage/storageAccounts/blobServices@2023-05-01' = {
  parent: storageAccount
  name: 'default'
  properties: {
    deleteRetentionPolicy: {
      enabled: true
      days: 14
    }
  }
}

resource backupContainer 'Microsoft.Storage/storageAccounts/blobServices/containers@2023-05-01' = {
  parent: blobService
  name: backupContainerName
  properties: {
    publicAccess: 'None'
  }
}

resource blobContributorRoleAssignment 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(storageAccount.id, vm.id, storageBlobDataContributorRoleId)
  scope: storageAccount
  properties: {
    roleDefinitionId: storageBlobDataContributorRoleId
    principalId: vm.identity.principalId
    principalType: 'ServicePrincipal'
  }
}

output vmName string = vm.name
output publicIpAddress string = publicIp.properties.ipAddress
output fqdn string = publicIp.properties.dnsSettings.fqdn
output dataDiskName string = dataDisk.name
output storageAccountName string = storageAccount.name
output backupContainerName string = backupContainer.name
output vmPrincipalId string = vm.identity.principalId
