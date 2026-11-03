import type {
  AutomationBusOverlay,
  CableChannelOverlay,
  DeviceRouteOverlay,
  Diagnostic,
  NetworkSummaryOverlay,
} from '@/lib/api'

export type BlockNetworkStatusKind =
  | 'online'
  | 'missing_channel'
  | 'insufficient_energy'
  | 'invalid_controller'
  | 'network_offline'

export interface BlockNetworkStatus {
  kind: BlockNetworkStatusKind
  label: string
  detail: string
}

interface BlockNetworkStatusInput {
  cable?: CableChannelOverlay
  route?: DeviceRouteOverlay
  bus?: AutomationBusOverlay
  network?: NetworkSummaryOverlay
  diagnostic?: Diagnostic
}

export function getBlockNetworkStatus({
  cable,
  route,
  bus,
  network,
  diagnostic,
}: BlockNetworkStatusInput): BlockNetworkStatus {
  const code = diagnostic?.code ?? ''
  const invalidController =
    Boolean(network?.has_controller && !network.controller_valid) ||
    /INVALID_CONTROLLER|MULTIPLE_CONTROLLERS|CONTROLLER_MULTIBLOCK/.test(code)
  if (invalidController) {
    return {
      kind: 'invalid_controller',
      label: 'Controller offline',
      detail: 'The ME Controller multiblock is invalid or disconnected.',
    }
  }

  const missingChannel =
    cable?.overloaded ||
    route?.has_channel === false ||
    bus?.has_channel === false ||
    /NO_CHANNEL|CHANNEL_CAPACITY/.test(code)
  if (missingChannel) {
    return {
      kind: 'missing_channel',
      label: 'Missing channel',
      detail: 'No AE2 channel is reaching this block.',
    }
  }

  const offline =
    cable?.online === false ||
    route?.online === false ||
    bus?.online === false ||
    network?.online === false
  if (offline) {
    if (/INSUFFICIENT_ENERGY|ENERGY/.test(code)) {
      return {
        kind: 'insufficient_energy',
        label: 'Insufficient energy',
        detail: 'The network is offline until its AE energy buffer recovers.',
      }
    }
    return {
      kind: 'network_offline',
      label: 'Network offline',
      detail: 'The AE2 network is not powered or cannot route this block.',
    }
  }

  if (/INSUFFICIENT_ENERGY|ENERGY/.test(code)) {
    return {
      kind: 'insufficient_energy',
      label: 'Insufficient energy',
      detail: 'The network is offline until its AE energy buffer recovers.',
    }
  }

  return {
    kind: 'online',
    label: 'Online',
    detail: 'This block is connected to the live AE2 network.',
  }
}
