type Listener = (state: { isReconnecting: boolean }) => void;
const listeners = new Set<Listener>();
let isReconnecting = false;

export function setSocketReconnecting(value: boolean) {
  if (isReconnecting === value) return;
  isReconnecting = value;
  listeners.forEach((l) => l({ isReconnecting }));
}

export function getSocketReconnecting() {
  return isReconnecting;
}

export function subscribeSocketStore(listener: Listener) {
  listeners.add(listener);
  listener({ isReconnecting });
  return () => {
    listeners.delete(listener);
  };
}
