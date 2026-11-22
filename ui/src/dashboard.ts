import { html, css, nothing } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import {
  LustrousElement,
  lustrousBase,
  lustrousFocusRing,
  lustrousType,
  resolveLustrousTheme,
  setLustrousTheme,
  type LustrousThemePreference,
} from '@ghchinoy/lustrous-lit/theme';
import '@ghchinoy/lustrous-lit/components/lustrous-badge';
import '@ghchinoy/lustrous-lit/components/lustrous-banner';
import '@ghchinoy/lustrous-lit/components/lustrous-button';
import '@ghchinoy/lustrous-lit/components/lustrous-collapsible-card';
import '@ghchinoy/lustrous-lit/components/lustrous-icon';
import '@ghchinoy/lustrous-lit/components/lustrous-icon-button';
import '@ghchinoy/lustrous-lit/components/lustrous-skeleton';
import '@ghchinoy/lustrous-lit/components/lustrous-snackbar';
import '@ghchinoy/lustrous-lit/components/lustrous-spinner';
import '@ghchinoy/lustrous-lit/components/lustrous-text';
import { showSnack } from '@ghchinoy/lustrous-lit/components/lustrous-snackbar';
import { homectlApi } from './api';
import type {
  LutronDevice,
  SonosDevice,
  SonosStatus,
  Camera,
  CastDevice,
  CastStatus,
  SonosFavorite,
  SonosService,
  RoborockDevice,
} from './api';

import './components/lutron-card';
import './components/sonos-card';
import './components/sonos-source-panel';
import './components/camera-card';
import './components/cast-card';
import './components/roborock-card';

const THEME_STORAGE_KEY = 'homectl-theme';

type SubsystemKey = 'lutron' | 'sonos' | 'cast' | 'cameras' | 'roborock';

@customElement('homectl-dashboard')
export class HomectlDashboard extends LustrousElement {
  @state() private lutronDevices: LutronDevice[] = [];
  @state() private lutronStatus: Record<string, number> = {};
  @state() private allLightsLevel = 0;
  @state() private sonosDevices: SonosDevice[] = [];
  @state() private sonosStatus: Record<string, SonosStatus> = {};
  @state() private selectedSpeakerIp = '';
  @state() private sonosFavorites: SonosFavorite[] = [];
  @state() private sonosServices: SonosService[] = [];
  @state() private sonosDefaultService?: SonosService;
  @state() private loadingSonosSource = false;
  @state() private castDevices: CastDevice[] = [];
  @state() private castStatus: Record<string, CastStatus> = {};
  @state() private cameras: Camera[] = [];
  @state() private roborockDevices: RoborockDevice[] = [];

  @state() private booting = true;
  @state() private refreshing = false;
  @state() private subsystemLoading: Record<SubsystemKey, boolean> = {
    lutron: true,
    sonos: true,
    cast: true,
    cameras: true,
    roborock: true,
  };
  @state() private subsystemErrors: Partial<Record<SubsystemKey, string>> = {};
  @state() private effectiveTheme: 'light' | 'dark' = 'light';
  @state() private themePreference: LustrousThemePreference = 'system';
  @state() private activeSection: SubsystemKey = 'lutron';

  private pollTimer?: number;
  private darkMediaQuery?: MediaQueryList;
  private sectionObserver?: IntersectionObserver;
  private observedSections = new WeakSet<Element>();
  private readonly onSystemThemeChange = () => {
    this.syncResolvedTheme();
  };
  private readonly onStorageChange = (e: StorageEvent) => {
    if (e.key === THEME_STORAGE_KEY) {
      this.applyStoredTheme();
    }
  };

  static override styles = [
    lustrousBase,
    lustrousFocusRing,
    css`
      :host {
        display: block;
        min-height: 100dvh;
        background-color: var(--lustrous-color-background);
        color: var(--lustrous-color-on-background);
      }

      .app-shell {
        max-width: 1440px;
        margin: 0 auto;
        padding: var(--lustrous-space-xl) var(--lustrous-space-2xl) var(--lustrous-space-4xl);
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-3xl);
      }

      @media (max-width: 768px) {
        .app-shell {
          padding: var(--lustrous-space-l) var(--lustrous-space-l) var(--lustrous-space-3xl);
          gap: var(--lustrous-space-2xl);
        }
      }

      .header {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        justify-content: space-between;
        gap: var(--lustrous-space-l);
        padding-bottom: var(--lustrous-space-l);
        border-bottom: 1px solid var(--lustrous-color-outline-variant);
      }

      .brand {
        display: flex;
        align-items: center;
        gap: var(--lustrous-space-m);
      }

      .brand-mark {
        width: 40px;
        height: 40px;
        border-radius: var(--lustrous-radius-m);
        background-color: var(--lustrous-color-primary-container);
        color: var(--lustrous-color-on-primary-container);
        display: inline-flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
      }

      .brand-copy {
        display: flex;
        flex-direction: column;
        gap: 2px;
      }

      .brand-subtitle {
        color: var(--lustrous-text-muted);
        ${lustrousType('body-small')}
      }

      .header-actions {
        display: flex;
        align-items: center;
        gap: var(--lustrous-space-s);
      }

      .status-pill {
        display: inline-flex;
        align-items: center;
        gap: var(--lustrous-space-s);
        padding: var(--lustrous-space-xs) var(--lustrous-space-m);
        border-radius: var(--lustrous-radius-full);
        background-color: var(--lustrous-color-surface-container);
        color: var(--lustrous-text-muted);
        ${lustrousType('caption')}
      }

      /* Initial full-screen boot experience */
      .boot-panel {
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-2xl);
      }

      .boot-hero {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        text-align: center;
        gap: var(--lustrous-space-m);
        padding: var(--lustrous-space-3xl) var(--lustrous-space-xl);
        background-color: var(--lustrous-color-surface);
        border: 1px solid var(--lustrous-color-outline-variant);
        border-radius: var(--lustrous-radius-xl);
        box-shadow: var(--lustrous-elevation-level1);
      }

      .boot-progress-copy {
        color: var(--lustrous-text-muted);
        max-width: 460px;
        margin: 0;
        ${lustrousType('body-medium')}
      }

      .section {
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-l);
        scroll-margin-top: 84px;
      }

      /* State-aware sticky section jump bar (single-row horizontal pill nav) */
      .sticky-nav-wrap {
        position: sticky;
        top: 0;
        z-index: 20;
        container-type: scroll-state;
        container-name: section-nav;
        margin-inline: calc(var(--lustrous-space-m) * -1);
        padding: var(--lustrous-space-s) var(--lustrous-space-m);
      }

      .sticky-nav {
        display: flex;
        align-items: center;
        gap: var(--lustrous-space-s);
        overflow-x: auto;
        overflow-y: hidden;
        flex-wrap: nowrap;
        scrollbar-width: none;
        scroll-snap-type: inline proximity;
        padding: var(--lustrous-space-xs);
        border-radius: var(--lustrous-radius-full);
        background-color: color-mix(in srgb, var(--lustrous-color-surface) 90%, transparent);
        backdrop-filter: blur(10px);
        -webkit-backdrop-filter: blur(10px);
        border: 1px solid var(--lustrous-color-outline-variant);
        box-shadow: var(--lustrous-elevation-level1);
        transition:
          box-shadow var(--lustrous-duration-short200) var(--lustrous-easing-standard),
          background-color var(--lustrous-duration-short200) var(--lustrous-easing-standard);
      }

      .sticky-nav::-webkit-scrollbar {
        display: none;
      }

      .nav-pill {
        flex: 0 0 auto;
        scroll-snap-align: start;
      }

      .nav-pill-count {
        opacity: 0.75;
        font-variant-numeric: tabular-nums;
      }

      @container section-nav scroll-state(stuck: top) {
        .sticky-nav {
          background-color: color-mix(in srgb, var(--lustrous-color-surface-bright) 96%, transparent);
          box-shadow: var(--lustrous-elevation-level2);
        }
      }

      .off-lan-wrap {
        margin-top: var(--lustrous-space-s);
      }

      .section-header {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        justify-content: space-between;
        gap: var(--lustrous-space-m);
      }

      .section-title-row {
        display: flex;
        align-items: center;
        gap: var(--lustrous-space-s);
      }

      .grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
        gap: var(--lustrous-space-xl);
      }

      .skeleton-card {
        background-color: var(--lustrous-color-surface);
        border: 1px solid var(--lustrous-color-outline-variant);
        border-radius: var(--lustrous-radius-l);
        padding: var(--lustrous-space-xl);
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-m);
        min-height: 176px;
      }

      .empty-section {
        padding: var(--lustrous-space-2xl);
        border-radius: var(--lustrous-radius-l);
        border: 1px dashed var(--lustrous-color-outline-variant);
        background-color: var(--lustrous-color-surface-container-low);
        color: var(--lustrous-text-muted);
        ${lustrousType('body-medium')}
      }

      @media (forced-colors: active) {
        .boot-hero,
        .skeleton-card,
        .sticky-nav {
          border: 1px solid CanvasText;
          box-shadow: none;
        }
        .nav-pill[aria-current='location'] {
          outline: 2px solid Highlight;
          border-radius: var(--lustrous-radius-full);
        }
      }
    `,
  ];

  override connectedCallback() {
    super.connectedCallback();
    this.applyStoredTheme();
    this.darkMediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    this.darkMediaQuery.addEventListener('change', this.onSystemThemeChange);
    window.addEventListener('storage', this.onStorageChange);
    void this.fetchData();
    this.pollTimer = window.setInterval(() => {
      void this.fetchStatus();
    }, 10000);
  }

  override disconnectedCallback() {
    super.disconnectedCallback();
    if (this.pollTimer !== undefined) {
      window.clearInterval(this.pollTimer);
      this.pollTimer = undefined;
    }
    this.sectionObserver?.disconnect();
    this.sectionObserver = undefined;
    this.darkMediaQuery?.removeEventListener('change', this.onSystemThemeChange);
    window.removeEventListener('storage', this.onStorageChange);
  }

  protected override updated(): void {
    this.observeSections();
  }

  private observeSections() {
    if (this.booting || typeof IntersectionObserver === 'undefined') return;
    if (!this.sectionObserver) {
      this.sectionObserver = new IntersectionObserver(
        (entries) => {
          const visible = entries
            .filter((e) => e.isIntersecting)
            .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
          if (visible.length > 0) {
            const id = visible[0].target.getAttribute('data-section') as SubsystemKey | null;
            if (id) {
              this.activeSection = id;
            }
          }
        },
        {
          rootMargin: '-96px 0px -65% 0px',
          threshold: [0, 0.1],
        },
      );
    }
    const sections = this.renderRoot.querySelectorAll('section[data-section]');
    sections.forEach((el) => {
      if (!this.observedSections.has(el)) {
        this.observedSections.add(el);
        this.sectionObserver?.observe(el);
      }
    });
  }

  private jumpToSection(key: SubsystemKey) {
    this.activeSection = key;
    const target = this.renderRoot.querySelector<HTMLElement>(`section[data-section="${key}"]`);
    if (!target) return;
    const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
    target.scrollIntoView({
      behavior: reduced ? 'auto' : 'smooth',
      block: 'start',
    });
  }

  private applyStoredTheme() {
    let saved: string | null = null;
    try {
      saved = window.localStorage.getItem(THEME_STORAGE_KEY);
    } catch {
      /* ignore */
    }
    const pref: LustrousThemePreference = saved === 'light' || saved === 'dark' ? saved : 'system';
    this.themePreference = pref;
    setLustrousTheme(pref);
    this.syncColorSchemeMeta(pref);
    this.syncResolvedTheme();
  }

  private syncColorSchemeMeta(pref: LustrousThemePreference) {
    const meta = document.querySelector<HTMLMetaElement>('meta[name="color-scheme"]');
    if (meta) {
      meta.content = pref === 'system' ? 'light dark' : pref;
    }
  }

  private syncResolvedTheme() {
    this.effectiveTheme = resolveLustrousTheme(document.documentElement);
  }

  /**
   * Two-state header theme toggle: switches to the opposite of the currently
   * rendered theme, storing a literal override only when it differs from OS
   * preference and removing storage when returning to system default.
   */
  private _toggleTheme() {
    const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const systemScheme: 'light' | 'dark' = systemDark ? 'dark' : 'light';
    const targetScheme: 'light' | 'dark' = this.effectiveTheme === 'dark' ? 'light' : 'dark';
    const nextPref: LustrousThemePreference = targetScheme === systemScheme ? 'system' : targetScheme;

    this.themePreference = nextPref;
    setLustrousTheme(nextPref);
    this.syncColorSchemeMeta(nextPref);
    try {
      if (nextPref === 'system') {
        window.localStorage.removeItem(THEME_STORAGE_KEY);
      } else {
        window.localStorage.setItem(THEME_STORAGE_KEY, nextPref);
      }
    } catch {
      /* ignore storage errors */
    }
    this.syncResolvedTheme();
  }

  private get completedSubsystemsCount(): number {
    return Object.values(this.subsystemLoading).filter((loading) => !loading).length;
  }

  private get isAnySubsystemLoading(): boolean {
    return Object.values(this.subsystemLoading).some(Boolean);
  }

  private get lanRoborockDevices(): RoborockDevice[] {
    return this.roborockDevices.filter((d) => Boolean(d.remote_ip) || d.connected);
  }

  private get offLanRoborockDevices(): RoborockDevice[] {
    return this.roborockDevices.filter((d) => !d.remote_ip && !d.connected);
  }

  private get totalDeviceCount(): number {
    const lutronCount = this.lutronDevices.filter((d) => d.LocalZones && d.LocalZones.length > 0).length;
    return (
      lutronCount +
      this.sonosDevices.length +
      this.castDevices.length +
      this.lanRoborockDevices.length +
      this.cameras.length
    );
  }

  private markSubsystemDone(key: SubsystemKey, error?: unknown) {
    this.subsystemLoading = { ...this.subsystemLoading, [key]: false };
    if (error) {
      const msg = error instanceof Error ? error.message : String(error);
      this.subsystemErrors = { ...this.subsystemErrors, [key]: msg };
    } else if (this.subsystemErrors[key]) {
      const next = { ...this.subsystemErrors };
      delete next[key];
      this.subsystemErrors = next;
    }
    // Reveal the progressive dashboard as soon as the first subsystem completes
    if (this.booting) {
      this.booting = false;
    }
  }

  async fetchData() {
    this.refreshing = true;
    this.subsystemLoading = {
      lutron: true,
      sonos: true,
      cast: true,
      cameras: true,
      roborock: true,
    };

    const lutronTask = homectlApi
      .getLutronDevices()
      .then(async (devices) => {
        this.lutronDevices = devices || [];
        this.markSubsystemDone('lutron');
        await this.fetchLutronStatus();
      })
      .catch((err) => {
        console.warn('Lutron discovery error:', err);
        this.markSubsystemDone('lutron', err);
      });

    const sonosTask = homectlApi
      .getSonosDevices()
      .then(async (devices) => {
        this.sonosDevices = devices || [];
        this.markSubsystemDone('sonos');
        if (this.sonosDevices.length > 0 && !this.selectedSpeakerIp) {
          this.selectedSpeakerIp = this.sonosDevices[0].IP;
          void this.fetchSonosSourceData(this.selectedSpeakerIp);
        }
        await this.fetchSonosStatus();
      })
      .catch((err) => {
        console.warn('Sonos discovery error:', err);
        this.markSubsystemDone('sonos', err);
      });

    const castTask = homectlApi
      .getCastDevices()
      .then((devices) => {
        this.castDevices = devices || [];
        this.markSubsystemDone('cast');
        this.fetchCastStatuses();
      })
      .catch((err) => {
        console.warn('Cast discovery error:', err);
        this.markSubsystemDone('cast', err);
      });

    const cameraTask = homectlApi
      .getCameras()
      .then((cameras) => {
        this.cameras = cameras || [];
        this.markSubsystemDone('cameras');
      })
      .catch((err) => {
        console.warn('Camera discovery error:', err);
        this.markSubsystemDone('cameras', err);
      });

    const roborockTask = homectlApi
      .getRoborockDevices()
      .then((res) => {
        this.roborockDevices = res.devices || [];
        this.markSubsystemDone('roborock');
      })
      .catch(() => {
        // Roborock is optional when unconfigured; treat gracefully as empty
        this.roborockDevices = [];
        this.markSubsystemDone('roborock');
      });

    await Promise.allSettled([lutronTask, sonosTask, castTask, cameraTask, roborockTask]);
    this.booting = false;
    this.refreshing = false;
  }

  private async fetchLutronStatus() {
    try {
      const lutron = await homectlApi.getLutronStatus();
      const newLutronStatus: Record<string, number> = {};
      lutron.forEach((s: { Zone: { href: string }; Level: number }) => {
        const href = s.Zone.href.replace('/status', '');
        newLutronStatus[href] = s.Level;
      });
      this.lutronStatus = newLutronStatus;
    } catch (e) {
      console.warn('Lutron status error:', e);
    }
  }

  private async fetchSonosStatus() {
    try {
      this.sonosStatus = await homectlApi.getSonosStatus();
    } catch (e) {
      console.warn('Sonos status error:', e);
    }
  }

  private fetchCastStatuses() {
    for (const d of this.castDevices) {
      homectlApi
        .getCastStatus(d.ip)
        .then((status) => {
          this.castStatus = { ...this.castStatus, [d.ip]: status };
        })
        .catch((e) => console.warn('Cast status error for', d.ip, e));
    }
  }

  async fetchStatus() {
    if (this.booting) return;
    await Promise.allSettled([
      this.fetchSonosStatus(),
      this.fetchLutronStatus(),
      homectlApi
        .getRoborockDevices()
        .then((res) => {
          this.roborockDevices = res.devices || [];
        })
        .catch(() => {}),
    ]);
    this.fetchCastStatuses();
  }

  async handleLutronChange(href: string, level: number) {
    this.lutronStatus = { ...this.lutronStatus, [href]: level };
    try {
      await homectlApi.setLutronLevel(href, level);
    } catch (e) {
      console.error('Failed to set level:', e);
      showSnack('Failed to update Lutron light level', { variant: 'error' });
      await this.fetchLutronStatus();
    }
  }

  async handleAllLutronChange(level: number) {
    this.allLightsLevel = level;
    const nextStatus = { ...this.lutronStatus };
    Object.keys(nextStatus).forEach((k) => {
      nextStatus[k] = level;
    });
    this.lutronStatus = nextStatus;

    try {
      await homectlApi.setAllLutronLevel(level);
      window.setTimeout(() => void this.fetchLutronStatus(), 1000);
    } catch (e) {
      console.error('All lights error:', e);
      showSnack('Failed to update all lights', { variant: 'error' });
    }
  }

  async handleSonosControl(ip: string, action: string) {
    try {
      await homectlApi.controlSonos(ip, action);
      window.setTimeout(() => void this.fetchSonosStatus(), 500);
    } catch (e) {
      console.error('Sonos control error:', e);
      showSnack(`Sonos ${action} failed`, { variant: 'error' });
    }
  }

  async handleSonosVolume(ip: string, volume: number) {
    this.sonosStatus = {
      ...this.sonosStatus,
      [ip]: { ...this.sonosStatus[ip], volume },
    };
    try {
      await homectlApi.controlSonos(ip, 'volume', volume);
    } catch (e) {
      console.error('Volume error:', e);
      showSnack('Failed to set Sonos volume', { variant: 'error' });
    }
  }

  async fetchSonosSourceData(ip: string) {
    if (!ip) return;
    this.loadingSonosSource = true;
    try {
      const [favResult, svcResult] = await Promise.all([
        homectlApi.getSonosFavorites(ip).catch(() => ({ count: 0, favorites: [] })),
        homectlApi.getSonosServices(ip).catch(() => ({ count: 0, services: [], default: undefined })),
      ]);
      this.sonosFavorites = favResult.favorites || [];
      this.sonosServices = svcResult.services || [];
      this.sonosDefaultService = svcResult.default;
    } catch (e) {
      console.warn('Failed to load Sonos source data for', ip, e);
    } finally {
      this.loadingSonosSource = false;
    }
  }

  async handleSpeakerChange(ip: string) {
    this.selectedSpeakerIp = ip;
    await this.fetchSonosSourceData(ip);
  }

  async handlePlayFavorite(ip: string, favoriteId: string, title?: string) {
    try {
      await homectlApi.playSonosFavorite(ip, favoriteId);
      showSnack(title ? `Playing "${title}"` : 'Playing Sonos favorite', { variant: 'positive' });
      window.setTimeout(() => void this.fetchSonosStatus(), 1000);
    } catch (e) {
      console.error('Play favorite error:', e);
      showSnack('Failed to play Sonos favorite', { variant: 'error' });
    }
  }

  async handlePlayStream(ip: string, url: string, title?: string) {
    try {
      await homectlApi.playSonosStream(ip, url, title);
      showSnack(title ? `Streaming "${title}"` : 'Started audio stream', { variant: 'positive' });
      window.setTimeout(() => void this.fetchSonosStatus(), 1000);
    } catch (e) {
      console.error('Play stream error:', e);
      showSnack('Failed to start audio stream', { variant: 'error' });
    }
  }

  async handleQueueAdd(ip: string, uri: string, asNext?: boolean) {
    try {
      await homectlApi.addSonosToQueue(ip, uri, asNext);
      showSnack('Added stream to Sonos queue', { variant: 'positive' });
      window.setTimeout(() => void this.fetchSonosStatus(), 1000);
    } catch (e) {
      console.error('Queue add error:', e);
      showSnack('Failed to add stream to queue', { variant: 'error' });
    }
  }

  async handleCastControl(ip: string, action: string) {
    try {
      await homectlApi.controlCast(ip, action);
      window.setTimeout(() => this.fetchCastStatuses(), 1000);
    } catch (e) {
      console.error('Cast control error:', e);
      showSnack(`Cast ${action} failed`, { variant: 'error' });
    }
  }

  async handleCastVolume(ip: string, volume: number) {
    this.castStatus = {
      ...this.castStatus,
      [ip]: { ...this.castStatus[ip], volume },
    };
    try {
      await homectlApi.controlCast(ip, 'volume', volume);
    } catch (e) {
      console.error('Cast volume error:', e);
      showSnack('Failed to set Cast volume', { variant: 'error' });
    }
  }

  async handleRoborockControl(id: string, action: string) {
    try {
      await homectlApi.controlRoborock(id, action);
      showSnack(`Sent "${action}" to Roborock`, { variant: 'positive' });
      window.setTimeout(() => void this.fetchStatus(), 1000);
    } catch (e) {
      console.error('Roborock control error:', e);
      showSnack(`Roborock ${action} failed`, { variant: 'error' });
    }
  }

  private renderSkeletonCards(count = 3) {
    return html`
      <div class="grid" aria-busy="true">
        ${Array.from({ length: count }).map(
          () => html`
            <div class="skeleton-card">
              <lustrous-skeleton width="84px" height="24px" border-radius="full"></lustrous-skeleton>
              <lustrous-skeleton width="65%" height="22px" border-radius="s"></lustrous-skeleton>
              <lustrous-skeleton width="45%" height="14px" border-radius="s"></lustrous-skeleton>
              <lustrous-skeleton width="100%" height="36px" border-radius="m"></lustrous-skeleton>
            </div>
          `,
        )}
      </div>
    `;
  }

  private renderSectionHeader(title: string, icon: 'lightbulb' | 'headset' | 'devices' | 'bolt' | 'photoCamera', loading: boolean, count: number) {
    return html`
      <div class="section-header">
        <div class="section-title-row">
          <lustrous-icon name=${icon} size="s" tone="primary"></lustrous-icon>
          <lustrous-text heading="2" variant="headline-small">${title}</lustrous-text>
          ${!loading
            ? html`<lustrous-badge tone="default">${count}</lustrous-badge>`
            : html`<lustrous-spinner variant="icon" size="xs" label="Discovering ${title}"></lustrous-spinner>`}
        </div>
      </div>
    `;
  }

  protected override render() {
    const lutronZones = this.lutronDevices.filter((d) => d.LocalZones && d.LocalZones.length > 0);
    const lanRoborocks = this.lanRoborockDevices;
    const offLanRoborocks = this.offLanRoborockDevices;
    const showRoborockSection = this.subsystemLoading.roborock || this.roborockDevices.length > 0;
    const errorEntries = Object.entries(this.subsystemErrors);
    const nextThemeLabel =
      this.effectiveTheme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme';

    const navItems: Array<{
      key: SubsystemKey;
      label: string;
      icon: 'lightbulb' | 'headset' | 'devices' | 'bolt' | 'photoCamera';
      count: number;
      loading: boolean;
    }> = [
      { key: 'lutron', label: 'Lighting', icon: 'lightbulb', count: lutronZones.length, loading: this.subsystemLoading.lutron },
      { key: 'sonos', label: 'Music', icon: 'headset', count: this.sonosDevices.length, loading: this.subsystemLoading.sonos },
      { key: 'cast', label: 'Video & Cast', icon: 'devices', count: this.castDevices.length, loading: this.subsystemLoading.cast },
      ...(showRoborockSection
        ? [{ key: 'roborock' as const, label: 'Roborock', icon: 'bolt' as const, count: lanRoborocks.length, loading: this.subsystemLoading.roborock }]
        : []),
      { key: 'cameras', label: 'Cameras', icon: 'photoCamera', count: this.cameras.length, loading: this.subsystemLoading.cameras },
    ];

    return html`
      <div class="app-shell">
        <header class="header">
          <div class="brand">
            <span class="brand-mark" aria-hidden="true">
              <lustrous-icon name="home" size="m"></lustrous-icon>
            </span>
            <div class="brand-copy">
              <lustrous-text heading="1" variant="headline-medium">homectl</lustrous-text>
              <span class="brand-subtitle">
                ${this.booting
                  ? 'Discovering smart home devices...'
                  : this.isAnySubsystemLoading
                    ? `${this.totalDeviceCount} devices found · scanning remaining subsystems (${this.completedSubsystemsCount}/5)`
                    : `${this.totalDeviceCount} connected devices across your home`}
              </span>
            </div>
          </div>

          <div class="header-actions">
            ${this.refreshing || this.isAnySubsystemLoading
              ? html`
                  <span class="status-pill" aria-live="polite">
                    <lustrous-spinner variant="icon" size="xs" label="Scanning network"></lustrous-spinner>
                    <span>Scanning (${this.completedSubsystemsCount}/5)</span>
                  </span>
                `
              : nothing}

            <lustrous-icon-button
              icon="refresh"
              variant="on-surface-variant"
              size="m"
              aria-label="Refresh devices"
              ?disabled=${this.refreshing}
              @click=${() => void this.fetchData()}
            ></lustrous-icon-button>

            <lustrous-icon-button
              icon=${this.effectiveTheme === 'dark' ? 'lightMode' : 'darkMode'}
              variant="on-surface-variant"
              size="m"
              aria-label=${nextThemeLabel}
              title=${this.themePreference === 'system'
                ? `${nextThemeLabel} (currently following system)`
                : `${nextThemeLabel}`}
              @click=${this._toggleTheme}
            ></lustrous-icon-button>
          </div>
        </header>

        ${errorEntries.length > 0
          ? html`
              <lustrous-banner
                variant="warning"
                icon="warning"
                heading="Some device subsystems did not respond"
                description=${errorEntries.map(([k]) => k).join(', ') + ' could not be reached right now.'}
                primary-action="Retry"
                dismissible
                @lustrous-action=${() => void this.fetchData()}
              ></lustrous-banner>
            `
          : nothing}
        ${this.booting
          ? html`
              <main class="boot-panel" aria-busy="true">
                <section class="boot-hero">
                  <lustrous-spinner
                    variant="ring"
                    size="l"
                    label="Loading homectl devices"
                  ></lustrous-spinner>
                  <lustrous-text heading="2" variant="headline-small">
                    Discovering your smart home
                  </lustrous-text>
                  <p class="boot-progress-copy">
                    Connecting to Lutron lighting, Sonos speakers, Google Cast, security cameras, and
                    Roborock vacuums&hellip;
                  </p>
                </section>
                ${this.renderSkeletonCards(6)}
              </main>
            `
          : html`
              <div class="sticky-nav-wrap">
                <nav class="sticky-nav" aria-label="Dashboard sections">
                  ${navItems.map((item) => {
                    const isActive = this.activeSection === item.key;
                    return html`
                      <lustrous-button
                        class="nav-pill"
                        shape="pill"
                        size="s"
                        icon=${item.icon}
                        variant=${isActive ? 'tonal' : 'on-surface-variant'}
                        aria-current=${isActive ? 'location' : nothing}
                        @click=${() => this.jumpToSection(item.key)}
                      >
                        ${item.label}${!item.loading ? html` <span class="nav-pill-count">(${item.count})</span>` : nothing}
                      </lustrous-button>
                    `;
                  })}
                </nav>
              </div>

              <main class="boot-panel">
                <section class="section" id="section-lutron" data-section="lutron">
                  ${this.renderSectionHeader('Lighting', 'lightbulb', this.subsystemLoading.lutron, lutronZones.length)}
                  ${this.subsystemLoading.lutron
                    ? this.renderSkeletonCards(3)
                    : html`
                        <div class="grid">
                          <lutron-card
                            name="ALL LIGHTS"
                            subtitle="Master Control"
                            .level=${this.allLightsLevel}
                            isMaster
                            @level-change=${(e: CustomEvent<{ level: number }>) =>
                              void this.handleAllLutronChange(e.detail.level)}
                          ></lutron-card>

                          ${lutronZones.map((d) => {
                            const href = d.LocalZones![0].href;
                            return html`
                              <lutron-card
                                .name=${d.Nickname || d.FullyQualifiedName || d.Name}
                                .subtitle=${d.DeviceType}
                                .level=${this.lutronStatus[href] ?? 0}
                                @level-change=${(e: CustomEvent<{ level: number }>) =>
                                  void this.handleLutronChange(href, e.detail.level)}
                              ></lutron-card>
                            `;
                          })}
                        </div>
                      `}
                </section>

                <section class="section" id="section-sonos" data-section="sonos">
                  ${this.renderSectionHeader('Music', 'headset', this.subsystemLoading.sonos, this.sonosDevices.length)}
                  ${this.subsystemLoading.sonos
                    ? this.renderSkeletonCards(3)
                    : this.sonosDevices.length === 0
                      ? html`<div class="empty-section">No Sonos speakers discovered on the network.</div>`
                      : html`
                          <sonos-source-panel
                            .devices=${this.sonosDevices}
                            .selectedSpeakerIp=${this.selectedSpeakerIp}
                            .favorites=${this.sonosFavorites}
                            .services=${this.sonosServices}
                            .defaultService=${this.sonosDefaultService}
                            .loading=${this.loadingSonosSource}
                            @speaker-change=${(e: CustomEvent<{ ip: string }>) =>
                              void this.handleSpeakerChange(e.detail.ip)}
                            @play-favorite=${(e: CustomEvent<{ ip: string; favoriteId: string; title?: string }>) =>
                              void this.handlePlayFavorite(e.detail.ip, e.detail.favoriteId, e.detail.title)}
                            @play-stream=${(e: CustomEvent<{ ip: string; url: string; title?: string }>) =>
                              void this.handlePlayStream(e.detail.ip, e.detail.url, e.detail.title)}
                            @queue-add=${(e: CustomEvent<{ ip: string; uri: string; asNext?: boolean }>) =>
                              void this.handleQueueAdd(e.detail.ip, e.detail.uri, e.detail.asNext)}
                          ></sonos-source-panel>

                          <div class="grid">
                            ${this.sonosDevices.map(
                              (d) => html`
                                <sonos-card
                                  .name=${d.Nickname || d.Name}
                                  .status=${this.sonosStatus[d.IP]}
                                  .artUrl=${homectlApi.getArtUrl(d.IP, this.sonosStatus[d.IP]?.album_art)}
                                  @control-change=${(e: CustomEvent<{ action: string }>) =>
                                    void this.handleSonosControl(d.IP, e.detail.action)}
                                  @volume-change=${(e: CustomEvent<{ volume: number }>) =>
                                    void this.handleSonosVolume(d.IP, e.detail.volume)}
                                ></sonos-card>
                              `,
                            )}
                          </div>
                        `}
                </section>

                <section class="section" id="section-cast" data-section="cast">
                  ${this.renderSectionHeader('Video & Cast', 'devices', this.subsystemLoading.cast, this.castDevices.length)}
                  ${this.subsystemLoading.cast
                    ? this.renderSkeletonCards(3)
                    : this.castDevices.length === 0
                      ? html`<div class="empty-section">No Google Cast devices discovered on the network.</div>`
                      : html`
                          <div class="grid">
                            ${this.castDevices.map(
                              (d) => html`
                                <cast-card
                                  .name=${d.name}
                                  .model=${d.model}
                                  .status=${this.castStatus[d.ip]}
                                  @control-change=${(e: CustomEvent<{ action: string }>) =>
                                    void this.handleCastControl(d.ip, e.detail.action)}
                                  @volume-change=${(e: CustomEvent<{ volume: number }>) =>
                                    void this.handleCastVolume(d.ip, e.detail.volume)}
                                ></cast-card>
                              `,
                            )}
                          </div>
                        `}
                </section>

                ${showRoborockSection
                  ? html`
                      <section class="section" id="section-roborock" data-section="roborock">
                        ${this.renderSectionHeader(
                          'Roborock Vacuums',
                          'bolt',
                          this.subsystemLoading.roborock,
                          lanRoborocks.length,
                        )}
                        ${this.subsystemLoading.roborock
                          ? this.renderSkeletonCards(2)
                          : html`
                              ${lanRoborocks.length > 0
                                ? html`
                                    <div class="grid">
                                      ${lanRoborocks.map(
                                        (d) => html`
                                          <roborock-card
                                            .device=${d}
                                            @roborock-control=${(e: CustomEvent<{ id: string; action: string }>) =>
                                              void this.handleRoborockControl(e.detail.id, e.detail.action)}
                                          ></roborock-card>
                                        `,
                                      )}
                                    </div>
                                  `
                                : html`<div class="empty-section">No Roborock vacuums detected on this local network.</div>`}
                              ${offLanRoborocks.length > 0
                                ? html`
                                    <div class="off-lan-wrap">
                                      <lustrous-collapsible-card
                                        heading="Not on this network"
                                        icon="cloudOff"
                                        badge=${String(offLanRoborocks.length)}
                                        badge-tone="default"
                                        subtitle="Roborock account devices not discovered on this LAN"
                                        .open=${false}
                                        storage-key="homectl-roborock-off-lan"
                                      >
                                        <div class="grid">
                                          ${offLanRoborocks.map(
                                            (d) => html`
                                              <roborock-card
                                                .device=${d}
                                                @roborock-control=${(e: CustomEvent<{ id: string; action: string }>) =>
                                                  void this.handleRoborockControl(e.detail.id, e.detail.action)}
                                              ></roborock-card>
                                            `,
                                          )}
                                        </div>
                                      </lustrous-collapsible-card>
                                    </div>
                                  `
                                : nothing}
                            `}
                      </section>
                    `
                  : nothing}

                <section class="section" id="section-cameras" data-section="cameras">
                  ${this.renderSectionHeader(
                    'Security Cameras',
                    'photoCamera',
                    this.subsystemLoading.cameras,
                    this.cameras.length,
                  )}
                  ${this.subsystemLoading.cameras
                    ? this.renderSkeletonCards(2)
                    : this.cameras.length === 0
                      ? html`<div class="empty-section">No security cameras discovered on the network.</div>`
                      : html`
                          <div class="grid">
                            ${this.cameras.map(
                              (c) => html`<camera-card .name=${c.name} .ip=${c.ip}></camera-card>`,
                            )}
                          </div>
                        `}
                </section>
              </main>
            `}
      </div>

      <lustrous-snackbar position="bottom-end"></lustrous-snackbar>
    `;
  }
}
