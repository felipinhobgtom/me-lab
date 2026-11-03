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
var nginxHttpConfigBase64 = base64('''
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name _;
    client_max_body_size 2m;

    location ~ ^/(api/|docs|openapi\.json|health|ready) {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto http;
    }

    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto http;
    }
}
''')
var appDeployScript = format('''
#!/bin/bash
set -euo pipefail

if command -v cloud-init >/dev/null 2>&1; then
  cloud-init status --wait || true
fi

if ! command -v docker >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y docker.io docker-compose-v2
  systemctl enable --now docker
fi

for attempt in $(seq 1 180); do
  if docker info >/dev/null 2>&1; then
    break
  fi
  sleep 10
done

if ! docker info >/dev/null 2>&1; then
  echo 'Docker did not become ready on the VM.' >&2
  systemctl status docker --no-pager >&2 || true
  exit 1
fi
data_device="/dev/disk/azure/scsi1/lun0"
for attempt in $(seq 1 60); do
  if [ -b "$data_device" ]; then
    break
  fi
  sleep 2
done
if [ ! -b "$data_device" ]; then
  echo "Managed data disk is not available at $data_device." >&2
  exit 1
fi
if ! blkid "$data_device" >/dev/null 2>&1; then
  mkfs.ext4 -L melab-data "$data_device"
fi
mkdir -p /mnt/melab-data
disk_uuid="$(blkid -s UUID -o value "$data_device")"
if ! grep -q "$disk_uuid" /etc/fstab; then
  echo "UUID=$disk_uuid /mnt/melab-data ext4 defaults,nofail 0 2" >> /etc/fstab
fi
if ! mountpoint -q /mnt/melab-data; then
  mount /mnt/melab-data
fi
chown -R 10001:10001 /mnt/melab-data
chmod 0750 /mnt/melab-data
if ! command -v nginx >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y nginx
fi
rm -f /etc/nginx/sites-enabled/default
if [ -s /etc/nginx/sites-available/me-lab.conf ]; then
  ln -sfn /etc/nginx/sites-available/me-lab.conf /etc/nginx/sites-enabled/me-lab.conf
  if nginx -t; then
    echo 'Preserving existing valid Nginx TLS configuration.'
  else
    rm -f /etc/nginx/sites-enabled/me-lab.conf
    printf '%s' '{9}' | base64 -d > /etc/nginx/sites-available/me-lab-http.conf
    ln -sfn /etc/nginx/sites-available/me-lab-http.conf /etc/nginx/sites-enabled/me-lab-http.conf
    nginx -t
  fi
else
  printf '%s' '{9}' | base64 -d > /etc/nginx/sites-available/me-lab-http.conf
  ln -sfn /etc/nginx/sites-available/me-lab-http.conf /etc/nginx/sites-enabled/me-lab-http.conf
  nginx -t
fi
systemctl enable --now nginx
systemctl reload nginx
mkdir -p /opt/me-lab/deployments/azure
printf '%s' '{0}' | base64 -d > /opt/me-lab/deployments/azure/compose.prod.yaml

jwt_secret="$(printf '%s' '{1}' | base64 -d)"
cors_allowed_origins="$(printf '%s' '{2}' | base64 -d)"
github_repository="$(printf '%s' '{3}' | base64 -d)"
storage_account_name="$(printf '%s' '{4}' | base64 -d)"
backup_container_name="$(printf '%s' '{5}' | base64 -d)"
ghcr_token="$(printf '%s' '{6}' | base64 -d)"
ghcr_username="$(printf '%s' '{7}' | base64 -d)"
image_tag="$(printf '%s' '{8}' | base64 -d)"

if [ ! -s /opt/me-lab/.env ]; then
  umask 077
  printf '%s\n' \
    'PORT=8080' \
    'DATABASE_URL=/mnt/melab-data/me-lab.db' \
    "JWT_SECRET=$jwt_secret" \
    'JWT_EXPIRY_HOURS=24' \
    'LOG_LEVEL=info' \
    "CORS_ALLOWED_ORIGINS=$cors_allowed_origins" \
    "GITHUB_REPOSITORY=$github_repository" \
    "AZURE_STORAGE_ACCOUNT=$storage_account_name" \
    "AZURE_STORAGE_CONTAINER=$backup_container_name" \
    'AZURE_USE_MANAGED_IDENTITY=true' \
    'RETENTION_DAYS=30' \
    > /opt/me-lab/.env
  chmod 0600 /opt/me-lab/.env
fi

printf '%s' "$ghcr_token" | docker login ghcr.io --username "$ghcr_username" --password-stdin
cd /opt/me-lab
IMAGE_TAG="$image_tag" GITHUB_REPOSITORY="$github_repository" docker compose --env-file /opt/me-lab/.env -f deployments/azure/compose.prod.yaml pull
IMAGE_TAG="$image_tag" GITHUB_REPOSITORY="$github_repository" docker compose --env-file /opt/me-lab/.env -f deployments/azure/compose.prod.yaml up -d --remove-orphans
docker logout ghcr.io >/dev/null 2>&1 || true
''', composeFileBase64, base64(jwtSecret), base64(corsAllowedOrigins), base64(githubRepository), base64(storageAccountName), base64(backupContainerName), base64(ghcrPullToken), base64(ghcrUsername), base64(appImageTag), nginxHttpConfigBase64)
var appDeployCommand = format('printf %s {0} | base64 -d > /tmp/me-lab-deploy.sh && chmod 700 /tmp/me-lab-deploy.sh && /bin/bash /tmp/me-lab-deploy.sh', base64(appDeployScript))

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
