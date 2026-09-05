import axios from 'axios';

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

const api = axios.create({
  baseURL: '/api'
});

export const homectlApi = {
  // Lutron
  getLutronDevices: () => api.get<LutronDevice[]>('/lutron/devices').then(r => r.data),
  getLutronStatus: () => api.get<any[]>('/lutron/status').then(r => r.data),
  setLutronLevel: (href: string, level: number) => api.post('/lutron/set', { href, level }),
  setAllLutronLevel: (level: number) => api.post('/lutron/all', { level }),

  // Sonos
  getSonosDevices: () => api.get<SonosDevice[]>('/sonos/devices').then(r => r.data),
  getSonosStatus: () => api.get<Record<string, SonosStatus>>('/sonos/status').then(r => r.data),
  controlSonos: (ip: string, action: string, volume?: number) => 
    api.post('/sonos/control', { ip, action, volume }),
  getSonosFavorites: (ip?: string) => 
    api.get<SonosFavoritesResult>('/sonos/favorites', { params: ip ? { ip } : {} }).then(r => r.data),
  playSonosFavorite: (ip: string, favoriteId: string) => 
    api.post('/sonos/play-favorite', { ip, favorite_id: favoriteId }).then(r => r.data),
  getSonosServices: (ip?: string) => 
    api.get<SonosServicesResult>('/sonos/services', { params: ip ? { ip } : {} }).then(r => r.data),
  playSonosStream: (ip: string, url: string, title?: string) => 
    api.post('/sonos/play-stream', { ip, url, title }).then(r => r.data),
  addSonosToQueue: (ip: string, uri: string, asNext?: boolean, metadata?: string) => 
    api.post('/sonos/queue-add', { ip, uri, as_next: asNext, metadata }).then(r => r.data),
  getSonosQueue: (ip?: string, start?: number, count?: number) =>
    api.get<SonosQueueResult>('/sonos/queue', { params: { ip, start, count } }).then(r => r.data),
  seekSonosTrack: (ip: string, track: number) =>
    api.post('/sonos/control', { ip, action: 'seek_track', track }),
  seekSonosTime: (ip: string, target: string) =>
    api.post('/sonos/control', { ip, action: 'seek_time', target }),

  // Cameras
  getCameras: () => api.get<Camera[]>('/security/cameras').then(r => r.data),

  // Cast
  getCastDevices: () => api.get<CastDevice[]>('/cast/devices').then(r => r.data),
  getCastStatus: (ip: string) => api.get<CastStatus>('/cast/status', { params: { ip } }).then(r => r.data),
  controlCast: (ip: string, action: string, volume?: number) => 
    api.post('/cast/control', { ip, action, volume }),

  // Utils
  getArtUrl: (ip: string, path: string) => {
    if (!path) return '';
    return `/api/sonos/art?ip=${ip}&path=${encodeURIComponent(path)}`;
  }
};