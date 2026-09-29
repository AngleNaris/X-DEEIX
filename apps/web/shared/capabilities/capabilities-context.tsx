"use client";

import { ALL_ENABLED, type CapabilityFlags, type Feature } from "@deeix/core";
import * as React from "react";

import { getCapabilities } from "@/shared/api/capabilities";
import { registerFeatureDisabledListener, resolveApiBaseURL } from "@/shared/api/http-client";

// Server capabilities decide which features the UI offers. They are loaded once
// per server; nothing here looks at the platform (docs/ARCHITECTURE.md §4).

type CapabilitiesState = {
  flags: CapabilityFlags;
  /** False until the first response (or fallback) for the current server arrived. */
  loaded: boolean;
};

const CapabilitiesContext = React.createContext<CapabilitiesState>({ flags: ALL_ENABLED, loaded: false });

// One in-flight request per server, shared by every mounted provider. Requests
// bypass the HTTP cache, so this map is the only cache and clearing it re-fetches.
const requests = new Map<string, Promise<CapabilityFlags>>();
const invalidationListeners = new Set<() => void>();

function requestCapabilities(server: string): Promise<CapabilityFlags> {
  let request = requests.get(server);
  if (!request) {
    request = getCapabilities();
    requests.set(server, request);
  }
  return request;
}

// A feature.disabled response means the server's capabilities differ from the
// loaded copy; every provider re-fetches.
function invalidateCapabilities(): void {
  requests.clear();
  for (const listener of invalidationListeners) {
    listener();
  }
}

export function CapabilitiesProvider({ children }: { children: React.ReactNode }) {
  const server = resolveApiBaseURL();
  const [state, setState] = React.useState<CapabilitiesState>({ flags: ALL_ENABLED, loaded: false });

  React.useEffect(() => {
    registerFeatureDisabledListener(invalidateCapabilities);
  }, []);

  React.useEffect(() => {
    let active = true;
    const load = () => {
      void requestCapabilities(server).then((flags) => {
        if (active) {
          setState({ flags, loaded: true });
        }
      });
    };
    setState((current) => ({ ...current, loaded: false }));
    load();
    invalidationListeners.add(load);
    return () => {
      active = false;
      invalidationListeners.delete(load);
    };
  }, [server]);

  return <CapabilitiesContext.Provider value={state}>{children}</CapabilitiesContext.Provider>;
}

export function useCapabilities(): CapabilitiesState {
  return React.useContext(CapabilitiesContext);
}

export function useFeature(feature: Feature): boolean {
  return React.useContext(CapabilitiesContext).flags[feature];
}

/**
 * Render children only when the server offers `feature`. Until capabilities are
 * known the gate is open, so a slow server never hides what it does provide.
 */
export function FeatureGate({
  feature,
  fallback = null,
  children,
}: {
  feature: Feature;
  fallback?: React.ReactNode;
  children: React.ReactNode;
}) {
  const enabled = useFeature(feature);
  return <>{enabled ? children : fallback}</>;
}
