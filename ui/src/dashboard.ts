import { LitElement, html, css } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { homectlApi } from './api';
import type { LutronDevice, SonosDevice, SonosStatus, Camera, CastDevice, CastStatus, SonosFavorite, SonosService } from './api';

import './components/lutron-card';
import './components/sonos-card';
import './components/sonos-source-panel';
import './components/camera-card';
import './components/cast-card';

@customElement('homectl-dashboard')
export class HomectlDashboard extends LitElement {
  @state() private lutronDevices: LutronDevice[] = [];
  @state() private lutronStatus: Record<string, number> = {};
  @state() private allLightsLevel = 0;
  @state() private sonosDevices: SonosDevice[] = [];
  @state() private sonosStatus: Record<string, SonosStatus> = {};
  @state() private selectedSpeakerIp = '';
  @state() private sonosFavorites: SonosFavorite[] = [];
  @state() private sonosServices: SonosService[] = [];
  @state() private sonosDefaultService?: SonosService;
  @state() private castDevices: CastDevice[] = [];
  @state() private castStatus: Record<string, CastStatus> = {};
  @state() private cameras: Camera[] = [];
  @state() private loading = true;

  static styles = css`
    :host {
      display: block;
      font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
      padding: 20px;
      color: #2c3e50;
      background: #f8f9fa;
      min-height: 100vh;
    }
    .header {
      margin-bottom: 30px;
      border-bottom: 2px solid #dee2e6;
      padding-bottom: 15px;
    }
    h1 { margin: 0; font-weight: 300; }
    h2 { font-size: 1.25rem; font-weight: 600; margin-top: 0; }
    
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
      gap: 25px;
      margin-bottom: 40px;
    }
  `;

  async connectedCallback() {
    super.connectedCallback();
    await this.fetchData();
    setInterval(() => this.fetchStatus(), 10000);
  }

  async fetchData() {
    this.loading = true;
    try {
      const [lutron, sonos, cameras, cast] = await Promise.all([
        homectlApi.getLutronDevices(),
        homectlApi.getSonosDevices(),
        homectlApi.getCameras(),
        homectlApi.getCastDevices()
      ]);
      this.lutronDevices = lutron;
      this.sonosDevices = sonos;
      this.cameras = cameras;
      this.castDevices = cast;
      if (this.sonosDevices.length > 0 && !this.selectedSpeakerIp) {
        this.selectedSpeakerIp = this.sonosDevices[0].IP;
        await this.fetchSonosSourceData(this.selectedSpeakerIp);
      }
      await this.fetchStatus();
    } catch (error) {
      console.error('Fetch error:', error);
    } finally {
      this.loading = false;
    }
  }

  async fetchStatus() {
    try {
      const [sonos, lutron] = await Promise.all([
        homectlApi.getSonosStatus(),
        homectlApi.getLutronStatus()
      ]);
      this.sonosStatus = sonos;
      
      const newLutronStatus: Record<string, number> = {};
      lutron.forEach((s: any) => {
        const href = s.Zone.href.replace('/status', '');
        newLutronStatus[href] = s.Level;
      });
      this.lutronStatus = newLutronStatus;

      // Fetch Cast statuses individually (backend limits concurrency)
      for (const d of this.castDevices) {
        homectlApi.getCastStatus(d.ip).then(status => {
          this.castStatus = { ...this.castStatus, [d.ip]: status };
        }).catch(e => console.warn('Cast status error for', d.ip, e));
      }
    } catch (e) { console.error('Status error:', e); }
  }

  async handleLutronChange(href: string, level: number) {
    this.lutronStatus = { ...this.lutronStatus, [href]: level };
    try {
      await homectlApi.setLutronLevel(href, level);
    } catch (e) {
      console.error('Failed to set level:', e);
      await this.fetchStatus();
    }
  }

  async handleAllLutronChange(level: number) {
    this.allLightsLevel = level;
    const nextStatus = { ...this.lutronStatus };
    Object.keys(nextStatus).forEach(k => nextStatus[k] = level);
    this.lutronStatus = nextStatus;

    try {
      await homectlApi.setAllLutronLevel(level);
      setTimeout(() => this.fetchStatus(), 1000);
    } catch (e) { console.error('All lights error:', e); }
  }

  async handleSonosControl(ip: string, action: string) {
    try {
      await homectlApi.controlSonos(ip, action);
      setTimeout(() => this.fetchStatus(), 500);
    } catch (e) { console.error('Sonos control error:', e); }
  }

  async handleSonosVolume(ip: string, volume: number) {
    this.sonosStatus = { 
      ...this.sonosStatus, 
      [ip]: { ...this.sonosStatus[ip], volume } 
    };
    try {
      await homectlApi.controlSonos(ip, 'volume', volume);
    } catch (e) { console.error('Volume error:', e); }
  }

  async fetchSonosSourceData(ip: string) {
    if (!ip) return;
    try {
      const [favResult, svcResult] = await Promise.all([
        homectlApi.getSonosFavorites(ip).catch(() => ({ count: 0, favorites: [] })),
        homectlApi.getSonosServices(ip).catch(() => ({ count: 0, services: [], default: undefined }))
      ]);
      this.sonosFavorites = favResult.favorites || [];
      this.sonosServices = svcResult.services || [];
      this.sonosDefaultService = svcResult.default;
    } catch (e) {
      console.warn('Failed to load Sonos source data for', ip, e);
    }
  }

  async handleSpeakerChange(ip: string) {
    this.selectedSpeakerIp = ip;
    await this.fetchSonosSourceData(ip);
  }

  async handlePlayFavorite(ip: string, favoriteId: string) {
    try {
      await homectlApi.playSonosFavorite(ip, favoriteId);
      setTimeout(() => this.fetchStatus(), 1000);
    } catch (e) {
      console.error('Play favorite error:', e);
    }
  }

  async handlePlayStream(ip: string, url: string, title?: string) {
    try {
      await homectlApi.playSonosStream(ip, url, title);
      setTimeout(() => this.fetchStatus(), 1000);
    } catch (e) {
      console.error('Play stream error:', e);
    }
  }

  async handleQueueAdd(ip: string, uri: string, asNext?: boolean) {
    try {
      await homectlApi.addSonosToQueue(ip, uri, asNext);
      setTimeout(() => this.fetchStatus(), 1000);
    } catch (e) {
      console.error('Queue add error:', e);
    }
  }

  async handleCastControl(ip: string, action: string) {
    try {
      await homectlApi.controlCast(ip, action);
      setTimeout(() => this.fetchStatus(), 1000);
    } catch (e) { console.error('Cast control error:', e); }
  }

  async handleCastVolume(ip: string, volume: number) {
    this.castStatus = { 
      ...this.castStatus, 
      [ip]: { ...this.castStatus[ip], volume } 
    };
    try {
      await homectlApi.controlCast(ip, 'volume', volume);
    } catch (e) { console.error('Cast volume error:', e); }
  }

  render() {
    if (this.loading) return html`<div style="padding: 40px; text-align: center;">Loading homectl...</div>`;

    return html`
      <div class="header">
        <h1>homectl</h1>
      </div>

      <section>
        <h2>Lighting</h2>
        <div class="grid">
          <lutron-card 
            name="ALL LIGHTS" 
            subtitle="Master Control" 
            .level=${this.allLightsLevel} 
            isMaster
            @level-change=${(e: any) => this.handleAllLutronChange(e.detail.level)}>
          </lutron-card>

          ${this.lutronDevices.filter(d => d.LocalZones && d.LocalZones.length > 0).map(d => {
            const href = d.LocalZones![0].href;
            return html`
              <lutron-card 
                .name=${d.Nickname || d.FullyQualifiedName || d.Name} 
                .subtitle=${d.DeviceType} 
                .level=${this.lutronStatus[href] ?? 0}
                @level-change=${(e: any) => this.handleLutronChange(href, e.detail.level)}>
              </lutron-card>
            `;
          })}
        </div>
      </section>

      <section>
        <h2>Music</h2>

        ${this.sonosDevices.length > 0 ? html`
          <sonos-source-panel
            .devices=${this.sonosDevices}
            .selectedSpeakerIp=${this.selectedSpeakerIp}
            .favorites=${this.sonosFavorites}
            .services=${this.sonosServices}
            .defaultService=${this.sonosDefaultService}
            @speaker-change=${(e: any) => this.handleSpeakerChange(e.detail.ip)}
            @play-favorite=${(e: any) => this.handlePlayFavorite(e.detail.ip, e.detail.favoriteId)}
            @play-stream=${(e: any) => this.handlePlayStream(e.detail.ip, e.detail.url, e.detail.title)}
            @queue-add=${(e: any) => this.handleQueueAdd(e.detail.ip, e.detail.uri, e.detail.asNext)}>
          </sonos-source-panel>
        ` : ''}

        <div class="grid">
          ${this.sonosDevices.map(d => html`
            <sonos-card 
              .name=${d.Nickname || d.Name} 
              .status=${this.sonosStatus[d.IP]}
              .artUrl=${homectlApi.getArtUrl(d.IP, this.sonosStatus[d.IP]?.album_art)}
              @control-change=${(e: any) => this.handleSonosControl(d.IP, e.detail.action)}
              @volume-change=${(e: any) => this.handleSonosVolume(d.IP, e.detail.volume)}>
            </sonos-card>
          `)}
        </div>
      </section>

      <section>
        <h2>Video & Cast</h2>
        <div class="grid">
          ${this.castDevices.map(d => html`
            <cast-card 
              .name=${d.name} 
              .model=${d.model} 
              .status=${this.castStatus[d.ip]}
              @control-change=${(e: any) => this.handleCastControl(d.ip, e.detail.action)}
              @volume-change=${(e: any) => this.handleCastVolume(d.ip, e.detail.volume)}>
            </cast-card>
          `)}
        </div>
      </section>

      <section>
        <h2>Security Cameras</h2>
        <div class="grid">
          ${this.cameras.map(c => html`
            <camera-card .name=${c.name} .ip=${c.ip}></camera-card>
          `)}
        </div>
      </section>
    `;
  }
}