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

export interface SonosStatus {
  status: string;
  volume: number;
  title: string;
  artist: string;
  album: string;
  album_art: string;
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

  // Cameras
  getCameras: () => api.get<Camera[]>('/security/cameras').then(r => r.data),

  // Cast
  getCastDevices: () => api.get<CastDevice[]>('/cast/devices').then(r => r.data),

  // Utils
  getArtUrl: (ip: string, path: string) => {
    if (!path) return '';
    return `/api/sonos/art?ip=${ip}&path=${encodeURIComponent(path)}`;
  }
};
