export interface LutronDevice {
  FullyQualifiedName?: string;
  Name: string;
  Nickname?: string;
  ModelNumber: string;
  DeviceType: string;
  LocalZones?: Array<{ href: string }>;
}

export interface SonosDevice {
  Name: string;
  Nickname?: string;
  IP: string;
  ModelName: string;
}

export interface CastDevice {
  name: string;
  ip: string;
  model: string;
}

export interface CastStatus {
  app_id: string;
  display_name: string;
  volume: number;
  is_muted: boolean;
  status_text: string;
}

export interface SonosStatus {
  status: string;
  volume: number;
  title: string;
  artist: string;
  album: string;
  album_art: string;
}

export interface SonosFavorite {
  id: string;
  title: string;
  type: string;
  resource_uri: string;
  metadata?: string;
  album_art_uri?: string;
  description?: string;
}

export interface SonosFavoritesResult {
  count: number;
  favorites: SonosFavorite[];
}

export interface SonosService {
  id: string;
  name: string;
  version?: string;
  uri?: string;
  secure_uri?: string;
  capabilities?: string;
  is_default?: boolean;
}

export interface SonosServicesResult {
  count: number;
  default?: SonosService;
  services: SonosService[];
}

export interface QueueItem {
  position: number;
  track_id: string;
  title: string;
  artist: string;
  album: string;
  duration: string;
  uri: string;
  album_art_uri?: string;
}

export interface SonosQueueResult {
  items: QueueItem[];
  returned: number;
  total_matches: number;
  start_index: number;
}

export interface Camera {
  name: string;
  ip: string;
}

async function getJSON<T>(path: string, params?: Record<string, string | number | undefined>): Promise<T> {
  const url = new URL(`/api${path}`, window.location.origin);
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined) {
        url.searchParams.set(k, String(v));
      }
    }
  }
  const resp = await fetch(url.toString());
  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
  }
  return resp.json() as Promise<T>;
}

async function postJSON<T = unknown>(path: string, body: unknown): Promise<T> {
  const resp = await fetch(`/api${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status}: ${await resp.text()}`);
  }
  const text = await resp.text();
  return (text ? JSON.parse(text) : undefined) as T;
}

export const homectlApi = {
  // Lutron
  getLutronDevices: () => getJSON<LutronDevice[]>('/lutron/devices'),
  getLutronStatus: () => getJSON<any[]>('/lutron/status'),
  setLutronLevel: (href: string, level: number) => postJSON('/lutron/set', { href, level }),
  setAllLutronLevel: (level: number) => postJSON('/lutron/all', { level }),

  // Sonos
  getSonosDevices: () => getJSON<SonosDevice[]>('/sonos/devices'),
  getSonosStatus: () => getJSON<Record<string, SonosStatus>>('/sonos/status'),
  controlSonos: (ip: string, action: string, volume?: number) =>
    postJSON('/sonos/control', { ip, action, volume }),
  getSonosFavorites: (ip?: string) =>
    getJSON<SonosFavoritesResult>('/sonos/favorites', ip ? { ip } : undefined),
  playSonosFavorite: (ip: string, favoriteId: string) =>
    postJSON('/sonos/play-favorite', { ip, favorite_id: favoriteId }),
  getSonosServices: (ip?: string) =>
    getJSON<SonosServicesResult>('/sonos/services', ip ? { ip } : undefined),
  playSonosStream: (ip: string, url: string, title?: string) =>
    postJSON('/sonos/play-stream', { ip, url, title }),
  addSonosToQueue: (ip: string, uri: string, asNext?: boolean, metadata?: string) =>
    postJSON('/sonos/queue-add', { ip, uri, as_next: asNext, metadata }),
  getSonosQueue: (ip?: string, start?: number, count?: number) =>
    getJSON<SonosQueueResult>('/sonos/queue', { ip, start, count }),
  seekSonosTrack: (ip: string, track: number) =>
    postJSON('/sonos/control', { ip, action: 'seek_track', track }),
  seekSonosTime: (ip: string, target: string) =>
    postJSON('/sonos/control', { ip, action: 'seek_time', target }),

  // Cameras
  getCameras: () => getJSON<Camera[]>('/security/cameras'),

  // Cast
  getCastDevices: () => getJSON<CastDevice[]>('/cast/devices'),
  getCastStatus: (ip: string) => getJSON<CastStatus>('/cast/status', { ip }),
  controlCast: (ip: string, action: string, volume?: number) =>
    postJSON('/cast/control', { ip, action, volume }),

  // Utils
  getArtUrl: (ip: string, path: string) => {
    if (!path) return '';
    return `/api/sonos/art?ip=${ip}&path=${encodeURIComponent(path)}`;
  }
};