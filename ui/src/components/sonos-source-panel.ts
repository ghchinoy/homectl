import { html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import {
  LustrousElement,
  lustrousBase,
  lustrousFocusRing,
  lustrousStateLayer,
  lustrousType,
} from '@ghchinoy/lustrous-lit/theme';
import '@ghchinoy/lustrous-lit/components/lustrous-badge';
import '@ghchinoy/lustrous-lit/components/lustrous-banner';
import '@ghchinoy/lustrous-lit/components/lustrous-button';
import '@ghchinoy/lustrous-lit/components/lustrous-icon';
import '@ghchinoy/lustrous-lit/components/lustrous-select';
import '@ghchinoy/lustrous-lit/components/lustrous-spinner';
import '@ghchinoy/lustrous-lit/components/lustrous-tabs';
import '@ghchinoy/lustrous-lit/components/lustrous-text';
import '@ghchinoy/lustrous-lit/components/lustrous-text-input';
import type { SonosDevice, SonosFavorite, SonosService } from '../api';
import { homectlApi } from '../api';

type SourceTab = 'favorites' | 'stream' | 'services';

@customElement('sonos-source-panel')
export class SonosSourcePanel extends LustrousElement {
  @property({ type: Array }) devices: SonosDevice[] = [];
  @property({ type: String }) selectedSpeakerIp = '';
  @property({ type: Array }) favorites: SonosFavorite[] = [];
  @property({ type: Array }) services: SonosService[] = [];
  @property({ type: Object }) defaultService?: SonosService;
  @property({ type: Boolean }) loading = false;

  @state() private searchQuery = '';
  @state() private activeTab: SourceTab = 'favorites';
  @state() private streamUrl = '';
  @state() private streamTitle = '';

  static override styles = [
    lustrousBase,
    lustrousFocusRing,
    lustrousStateLayer,
    css`
      :host {
        display: block;
        margin-bottom: var(--lustrous-space-2xl);
      }

      .panel {
        background-color: var(--lustrous-color-surface);
        color: var(--lustrous-color-on-surface);
        border: 1px solid var(--lustrous-color-outline-variant);
        border-radius: var(--lustrous-radius-l);
        padding: var(--lustrous-space-2xl);
        box-shadow: var(--lustrous-elevation-level1);
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-xl);
      }

      .panel-header {
        display: flex;
        flex-wrap: wrap;
        align-items: flex-end;
        justify-content: space-between;
        gap: var(--lustrous-space-l);
        padding-bottom: var(--lustrous-space-l);
        border-bottom: 1px solid var(--lustrous-color-outline-variant);
      }

      .header-title-group {
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-xs);
      }

      .speaker-selector {
        min-width: 240px;
        flex: 0 1 320px;
      }

      .search-bar-container {
        display: flex;
        flex-wrap: wrap;
        gap: var(--lustrous-space-m);
        align-items: center;
      }

      .search-field {
        flex: 1 1 260px;
      }

      .search-count {
        color: var(--lustrous-text-muted);
        white-space: nowrap;
        ${lustrousType('body-small')}
      }

      .loading-row {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: var(--lustrous-space-m);
        padding: var(--lustrous-space-3xl) var(--lustrous-space-l);
        color: var(--lustrous-text-muted);
        ${lustrousType('body-medium')}
      }

      .favorites-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
        gap: var(--lustrous-space-l);
      }

      .fav-card {
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-s);
        background-color: var(--lustrous-color-surface-container-low);
        border: 1px solid var(--lustrous-color-outline-variant);
        border-radius: var(--lustrous-radius-m);
        padding: var(--lustrous-space-m);
        transition:
          transform var(--lustrous-duration-short150) var(--lustrous-easing-standard),
          box-shadow var(--lustrous-duration-short150) var(--lustrous-easing-standard),
          border-color var(--lustrous-duration-short150) var(--lustrous-easing-standard);
      }

      .fav-card:hover {
        transform: translateY(-2px);
        box-shadow: var(--lustrous-elevation-level1);
        border-color: var(--lustrous-color-outline);
      }

      .fav-art-wrap {
        width: 100%;
        aspect-ratio: 1;
        border-radius: var(--lustrous-radius-s);
        overflow: hidden;
        background-color: var(--lustrous-color-surface-container);
        display: flex;
        align-items: center;
        justify-content: center;
      }

      .fav-art {
        width: 100%;
        height: 100%;
        object-fit: cover;
        display: block;
      }

      .fav-title {
        color: var(--lustrous-color-on-surface);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        ${lustrousType('body-medium-emphasized')}
      }

      .fav-desc {
        color: var(--lustrous-text-muted);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        ${lustrousType('caption')}
      }

      .fav-play-action {
        margin-top: auto;
        width: 100%;
      }

      .stream-form {
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-l);
        max-width: 640px;
      }

      .btn-group {
        display: flex;
        flex-wrap: wrap;
        gap: var(--lustrous-space-m);
      }

      .table-wrap {
        overflow-x: auto;
        border: 1px solid var(--lustrous-color-outline-variant);
        border-radius: var(--lustrous-radius-m);
      }

      .services-table {
        width: 100%;
        border-collapse: collapse;
        ${lustrousType('body-small')}
      }

      .services-table th {
        text-align: start;
        padding: var(--lustrous-space-m) var(--lustrous-space-l);
        background-color: var(--lustrous-color-surface-container);
        color: var(--lustrous-text-muted);
        border-bottom: 1px solid var(--lustrous-color-outline-variant);
        ${lustrousType('body-small-emphasized')}
      }

      .services-table td {
        padding: var(--lustrous-space-m) var(--lustrous-space-l);
        border-bottom: 1px solid var(--lustrous-color-outline-variant);
        color: var(--lustrous-color-on-surface);
      }

      .services-table tr:last-child td {
        border-bottom: 0;
      }

      .cell-strong {
        ${lustrousType('body-small-emphasized')}
      }

      .cell-muted {
        color: var(--lustrous-text-muted);
        font-family: var(--lustrous-font-code);
      }

      .services-stack {
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-l);
      }

      .empty-state {
        padding: var(--lustrous-space-3xl) var(--lustrous-space-l);
        text-align: center;
        color: var(--lustrous-text-muted);
        background-color: var(--lustrous-color-surface-container-low);
        border-radius: var(--lustrous-radius-m);
        border: 1px dashed var(--lustrous-color-outline-variant);
        ${lustrousType('body-medium')}
      }

      @media (forced-colors: active) {
        .panel,
        .fav-card,
        .table-wrap {
          border: 1px solid CanvasText;
          box-shadow: none;
        }
      }
    `,
  ];

  private _onSpeakerSelect(e: CustomEvent<{ value: string }>) {
    const ip = e.detail.value;
    this.dispatchEvent(
      new CustomEvent('speaker-change', {
        detail: { ip },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onTabChange(e: CustomEvent<{ value: string }>) {
    const next = e.detail.value;
    if (next === 'favorites' || next === 'stream' || next === 'services') {
      this.activeTab = next;
    }
  }

  private _onSearchInput(e: Event) {
    const target = e.currentTarget as HTMLElement & { value: string };
    this.searchQuery = target.value ?? '';
  }

  private _playFavorite(fav: SonosFavorite) {
    if (!this.selectedSpeakerIp) return;
    this.dispatchEvent(
      new CustomEvent('play-favorite', {
        detail: { ip: this.selectedSpeakerIp, favoriteId: fav.id, title: fav.title },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _playStream() {
    if (!this.selectedSpeakerIp || !this.streamUrl) return;
    this.dispatchEvent(
      new CustomEvent('play-stream', {
        detail: {
          ip: this.selectedSpeakerIp,
          url: this.streamUrl,
          title: this.streamTitle || undefined,
        },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _queueAdd() {
    if (!this.selectedSpeakerIp || !this.streamUrl) return;
    this.dispatchEvent(
      new CustomEvent('queue-add', {
        detail: {
          ip: this.selectedSpeakerIp,
          uri: this.streamUrl,
          asNext: true,
        },
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected override render() {
    const filteredFavorites = this.favorites.filter((f) => {
      if (!this.searchQuery) return true;
      const q = this.searchQuery.toLowerCase();
      return (
        (f.title && f.title.toLowerCase().includes(q)) ||
        (f.description && f.description.toLowerCase().includes(q)) ||
        (f.type && f.type.toLowerCase().includes(q))
      );
    });

    const speakerOptions = this.devices.map((d) => ({
      value: d.IP,
      label: d.Nickname || d.Name,
      description: d.IP,
      icon: 'headset' as const,
    }));

    const tabs = [
      { value: 'favorites', label: `Favorites (${this.favorites.length})`, icon: 'star' as const },
      { value: 'stream', label: 'Direct Stream', icon: 'link' as const },
      { value: 'services', label: `Music Services (${this.services.length})`, icon: 'layers' as const },
    ];

    return html`
      <div class="panel">
        <div class="panel-header">
          <div class="header-title-group">
            <lustrous-badge tone="amber" icon="headset">Sonos Source</lustrous-badge>
            <lustrous-text heading="3" variant="headline-small">Music Catalog &amp; Streams</lustrous-text>
          </div>

          <div class="speaker-selector">
            <lustrous-select
              label="Target Speaker"
              size="s"
              .options=${speakerOptions}
              .value=${this.selectedSpeakerIp}
              @lustrous-change=${this._onSpeakerSelect}
            ></lustrous-select>
          </div>
        </div>

        <lustrous-tabs
          variant="underline"
          label="Sonos audio sources"
          .tabs=${tabs}
          .value=${this.activeTab}
          @lustrous-change=${this._onTabChange}
        ></lustrous-tabs>

        ${this.loading
          ? html`
              <div class="loading-row" aria-busy="true">
                <lustrous-spinner variant="ring" size="s" label="Loading speaker catalog"></lustrous-spinner>
                <span>Loading favorites and music services&hellip;</span>
              </div>
            `
          : nothing}
        ${!this.loading && this.activeTab === 'favorites'
          ? html`
              <div class="search-bar-container">
                <lustrous-text-input
                  class="search-field"
                  type="search"
                  size="s"
                  leading-icon="search"
                  aria-label="Search Sonos favorites"
                  placeholder="Search favorites, playlists, albums, radio stations..."
                  .value=${this.searchQuery}
                  @input=${this._onSearchInput}
                ></lustrous-text-input>
                <span class="search-count">
                  ${this.searchQuery
                    ? `Showing ${filteredFavorites.length} of ${this.favorites.length}`
                    : `${this.favorites.length} total`}
                </span>
              </div>

              ${filteredFavorites.length === 0
                ? html`
                    <div class="empty-state">
                      ${this.searchQuery
                        ? `No favorites found matching "${this.searchQuery}".`
                        : 'No favorites pinned on this speaker yet. Pin playlists or radio stations in the Sonos app to access them here.'}
                    </div>
                  `
                : html`
                    <div class="favorites-grid">
                      ${filteredFavorites.map((fav) => {
                        const artUrl = fav.album_art_uri
                          ? homectlApi.getArtUrl(this.selectedSpeakerIp, fav.album_art_uri)
                          : '';
                        return html`
                          <div class="fav-card">
                            <div class="fav-art-wrap">
                              ${artUrl
                                ? html`<img class="fav-art" src=${artUrl} alt=${fav.title} loading="lazy" />`
                                : html`<lustrous-icon name="headset" size="l" muted></lustrous-icon>`}
                            </div>
                            <div class="fav-title" title=${fav.title}>${fav.title}</div>
                            <div class="fav-desc">${fav.description || 'Sonos Favorite'}</div>
                            <lustrous-button
                              class="fav-play-action"
                              variant="tonal"
                              size="xs"
                              icon="playArrow"
                              @click=${() => this._playFavorite(fav)}
                            >
                              Play
                            </lustrous-button>
                          </div>
                        `;
                      })}
                    </div>
                  `}
            `
          : nothing}
        ${!this.loading && this.activeTab === 'stream'
          ? html`
              <div class="stream-form">
                <lustrous-text-input
                  label="Audio Stream URL (HTTP/HTTPS)"
                  type="url"
                  leading-icon="link"
                  placeholder="https://stream.somafm.com/groovesalad-128-mp3"
                  .value=${this.streamUrl}
                  @input=${(e: Event) => {
                    this.streamUrl = (e.currentTarget as HTMLElement & { value: string }).value ?? '';
                  }}
                ></lustrous-text-input>

                <lustrous-text-input
                  label="Display Title (optional)"
                  type="text"
                  placeholder="e.g. SomaFM Groove Salad"
                  .value=${this.streamTitle}
                  @input=${(e: Event) => {
                    this.streamTitle = (e.currentTarget as HTMLElement & { value: string }).value ?? '';
                  }}
                ></lustrous-text-input>

                <div class="btn-group">
                  <lustrous-button
                    variant="accent"
                    size="s"
                    icon="playArrow"
                    ?disabled=${!this.streamUrl}
                    @click=${this._playStream}
                  >
                    Play Stream Now
                  </lustrous-button>
                  <lustrous-button
                    variant="outline"
                    size="s"
                    icon="add"
                    ?disabled=${!this.streamUrl}
                    @click=${this._queueAdd}
                  >
                    Add as Next in Queue
                  </lustrous-button>
                </div>
              </div>
            `
          : nothing}
        ${!this.loading && this.activeTab === 'services'
          ? html`
              <div class="services-stack">
                ${this.services.length === 0
                  ? html`<div class="empty-state">No music services reported by this speaker.</div>`
                  : html`
                      <div class="table-wrap">
                        <table class="services-table">
                          <thead>
                            <tr>
                              <th>Service Name</th>
                              <th>ID</th>
                              <th>Status</th>
                              <th>Version</th>
                            </tr>
                          </thead>
                          <tbody>
                            ${this.services.map(
                              (s) => html`
                                <tr>
                                  <td class="cell-strong">${s.name}</td>
                                  <td class="cell-muted">${s.id}</td>
                                  <td>
                                    ${s.is_default
                                      ? html`<lustrous-badge tone="positive">Default</lustrous-badge>`
                                      : nothing}
                                  </td>
                                  <td class="cell-muted">${s.version || '1.1'}</td>
                                </tr>
                              `,
                            )}
                          </tbody>
                        </table>
                      </div>

                      <lustrous-banner
                        variant="info"
                        icon="info"
                        heading="Music Service Integration"
                        description="Sonos Favorites provide instant, credential-free playback of pinned cloud playlists and radio across all registered providers. The marked default service is used by agents when selecting ambient music without an explicit provider."
                      ></lustrous-banner>
                    `}
              </div>
            `
          : nothing}
      </div>
    `;
  }
}
