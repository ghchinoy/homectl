import { html, css, LitElement } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import type { SonosDevice, SonosFavorite, SonosService } from '../api';
import { homectlApi } from '../api';

@customElement('sonos-source-panel')
export class SonosSourcePanel extends LitElement {
  @property({ type: Array }) devices: SonosDevice[] = [];
  @property({ type: String }) selectedSpeakerIp = '';
  @property({ type: Array }) favorites: SonosFavorite[] = [];
  @property({ type: Array }) services: SonosService[] = [];
  @property({ type: Object }) defaultService?: SonosService;
  @property({ type: Boolean }) loading = false;

  @state() private searchQuery = '';
  @state() private activeTab: 'favorites' | 'stream' | 'services' = 'favorites';
  @state() private streamUrl = '';
  @state() private streamTitle = '';

  static styles = css`
    :host {
      display: block;
      margin-bottom: 30px;
    }
    .panel {
      background: #ffffff;
      border-radius: 12px;
      padding: 24px;
      box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06);
      border: 1px solid #edf2f7;
    }
    .panel-header {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 20px;
      padding-bottom: 16px;
      border-bottom: 1px solid #edf2f7;
    }
    .header-title-group {
      display: flex;
      align-items: center;
      gap: 12px;
    }
    .panel-badge {
      background: #fff3e0;
      color: #e65100;
      font-size: 0.75rem;
      font-weight: 700;
      text-transform: uppercase;
      padding: 4px 8px;
      border-radius: 4px;
      letter-spacing: 0.5px;
    }
    h3 {
      margin: 0;
      font-size: 1.2rem;
      font-weight: 600;
      color: #2c3e50;
    }
    .speaker-selector-group {
      display: flex;
      align-items: center;
      gap: 8px;
    }
    label {
      font-size: 0.85rem;
      color: #7f8c8d;
      font-weight: 500;
    }
    select {
      padding: 6px 12px;
      border-radius: 6px;
      border: 1px solid #cbd5e1;
      background: #fff;
      font-size: 0.9rem;
      color: #2c3e50;
      cursor: pointer;
      outline: none;
      transition: border-color 0.2s;
    }
    select:focus {
      border-color: #e67e22;
    }

    /* Tabs */
    .tabs {
      display: flex;
      gap: 8px;
      margin-bottom: 20px;
      border-bottom: 2px solid #f1f5f9;
      padding-bottom: 2px;
    }
    .tab-btn {
      background: none;
      border: none;
      padding: 8px 16px;
      font-size: 0.95rem;
      font-weight: 500;
      color: #64748b;
      cursor: pointer;
      border-radius: 6px 6px 0 0;
      position: relative;
      transition: all 0.2s;
    }
    .tab-btn:hover {
      color: #2c3e50;
      background: #f8fafc;
    }
    .tab-btn.active {
      color: #e67e22;
      font-weight: 600;
    }
    .tab-btn.active::after {
      content: '';
      position: absolute;
      bottom: -4px;
      left: 0;
      right: 0;
      height: 2px;
      background: #e67e22;
    }

    /* Search Bar */
    .search-bar-container {
      margin-bottom: 20px;
      display: flex;
      gap: 12px;
      align-items: center;
    }
    .search-input {
      flex: 1;
      padding: 10px 14px;
      border-radius: 8px;
      border: 1px solid #cbd5e1;
      font-size: 0.95rem;
      outline: none;
      transition: all 0.2s;
    }
    .search-input:focus {
      border-color: #e67e22;
      box-shadow: 0 0 0 3px rgba(230, 126, 34, 0.15);
    }
    .search-count {
      font-size: 0.85rem;
      color: #94a3b8;
      white-space: nowrap;
    }

    /* Favorites Grid */
    .favorites-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
      gap: 16px;
    }
    .fav-card {
      display: flex;
      flex-direction: column;
      background: #f8fafc;
      border: 1px solid #e2e8f0;
      border-radius: 8px;
      padding: 12px;
      cursor: pointer;
      transition: all 0.2s ease;
      position: relative;
    }
    .fav-card:hover {
      transform: translateY(-2px);
      box-shadow: 0 6px 12px rgba(0, 0, 0, 0.05);
      border-color: #cbd5e1;
      background: #ffffff;
    }
    .fav-art-wrap {
      width: 100%;
      aspect-ratio: 1;
      border-radius: 6px;
      overflow: hidden;
      background: #e2e8f0;
      margin-bottom: 10px;
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .fav-art {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }
    .fav-art-placeholder {
      font-size: 2.5rem;
      color: #94a3b8;
    }
    .fav-title {
      font-weight: 600;
      font-size: 0.9rem;
      color: #1e293b;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      margin-bottom: 4px;
    }
    .fav-desc {
      font-size: 0.75rem;
      color: #64748b;
      margin-bottom: 8px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .fav-play-btn {
      margin-top: auto;
      background: #e67e22;
      color: white;
      border: none;
      padding: 6px 12px;
      border-radius: 6px;
      font-size: 0.8rem;
      font-weight: 600;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 6px;
      transition: background 0.2s;
    }
    .fav-play-btn:hover {
      background: #d35400;
    }

    /* Stream Form */
    .form-group {
      margin-bottom: 16px;
    }
    .form-label {
      display: block;
      font-size: 0.85rem;
      font-weight: 600;
      color: #475569;
      margin-bottom: 6px;
    }
    .text-input {
      width: 100%;
      box-sizing: border-box;
      padding: 10px 14px;
      border-radius: 8px;
      border: 1px solid #cbd5e1;
      font-size: 0.95rem;
      outline: none;
    }
    .text-input:focus {
      border-color: #e67e22;
      box-shadow: 0 0 0 3px rgba(230, 126, 34, 0.15);
    }
    .btn-group {
      display: flex;
      gap: 12px;
      margin-top: 20px;
    }
    .btn-primary {
      background: #e67e22;
      color: white;
      border: none;
      padding: 10px 20px;
      border-radius: 6px;
      font-size: 0.9rem;
      font-weight: 600;
      cursor: pointer;
      transition: background 0.2s;
    }
    .btn-primary:hover {
      background: #d35400;
    }
    .btn-secondary {
      background: #f1f5f9;
      color: #334155;
      border: 1px solid #cbd5e1;
      padding: 10px 20px;
      border-radius: 6px;
      font-size: 0.9rem;
      font-weight: 600;
      cursor: pointer;
      transition: background 0.2s;
    }
    .btn-secondary:hover {
      background: #e2e8f0;
    }

    /* Services Table */
    .services-table {
      width: 100%;
      border-collapse: collapse;
      font-size: 0.9rem;
    }
    .services-table th {
      text-align: left;
      padding: 10px 12px;
      background: #f8fafc;
      color: #64748b;
      font-weight: 600;
      border-bottom: 2px solid #e2e8f0;
    }
    .services-table td {
      padding: 10px 12px;
      border-bottom: 1px solid #f1f5f9;
      color: #1e293b;
    }
    .default-tag {
      background: #dcfce7;
      color: #15803d;
      font-size: 0.75rem;
      font-weight: 700;
      padding: 3px 8px;
      border-radius: 12px;
      display: inline-block;
    }
    .service-info-callout {
      margin-top: 16px;
      padding: 12px 16px;
      background: #f8fafc;
      border-radius: 8px;
      border-left: 4px solid #e67e22;
      font-size: 0.85rem;
      color: #64748b;
      line-height: 1.5;
    }

    .empty-state {
      padding: 40px;
      text-align: center;
      color: #94a3b8;
      font-size: 0.95rem;
    }
  `;

  private _onSpeakerSelect(e: Event) {
    const target = e.target as HTMLSelectElement;
    const ip = target.value;
    this.dispatchEvent(new CustomEvent('speaker-change', {
      detail: { ip },
      bubbles: true,
      composed: true
    }));
  }

  private _onSearchChange(e: Event) {
    const target = e.target as HTMLInputElement;
    this.searchQuery = target.value;
  }

  private _playFavorite(fav: SonosFavorite) {
    if (!this.selectedSpeakerIp) return;
    this.dispatchEvent(new CustomEvent('play-favorite', {
      detail: { ip: this.selectedSpeakerIp, favoriteId: fav.id, title: fav.title },
      bubbles: true,
      composed: true
    }));
  }

  private _playStream() {
    if (!this.selectedSpeakerIp || !this.streamUrl) return;
    this.dispatchEvent(new CustomEvent('play-stream', {
      detail: {
        ip: this.selectedSpeakerIp,
        url: this.streamUrl,
        title: this.streamTitle || undefined
      },
      bubbles: true,
      composed: true
    }));
  }

  private _queueAdd() {
    if (!this.selectedSpeakerIp || !this.streamUrl) return;
    this.dispatchEvent(new CustomEvent('queue-add', {
      detail: {
        ip: this.selectedSpeakerIp,
        uri: this.streamUrl,
        asNext: true
      },
      bubbles: true,
      composed: true
    }));
  }

  render() {
    const filteredFavorites = this.favorites.filter(f => {
      if (!this.searchQuery) return true;
      const q = this.searchQuery.toLowerCase();
      return (f.title && f.title.toLowerCase().includes(q)) ||
             (f.description && f.description.toLowerCase().includes(q)) ||
             (f.type && f.type.toLowerCase().includes(q));
    });

    return html`
      <div class="panel">
        <div class="panel-header">
          <div class="header-title-group">
            <span class="panel-badge">Sonos Source</span>
            <h3>Music Catalog & Streams</h3>
          </div>

          <div class="speaker-selector-group">
            <label for="speaker-select">Target Speaker:</label>
            <select id="speaker-select" .value=${this.selectedSpeakerIp} @change=${this._onSpeakerSelect}>
              ${this.devices.map(d => html`
                <option value=${d.IP} ?selected=${d.IP === this.selectedSpeakerIp}>
                  ${d.Nickname || d.Name} (${d.IP})
                </option>
              `)}
            </select>
          </div>
        </div>

        <div class="tabs">
          <button 
            class="tab-btn ${this.activeTab === 'favorites' ? 'active' : ''}" 
            @click=${() => this.activeTab = 'favorites'}>
            Favorites & Playlists (${this.favorites.length})
          </button>
          <button 
            class="tab-btn ${this.activeTab === 'stream' ? 'active' : ''}" 
            @click=${() => this.activeTab = 'stream'}>
            Direct Audio Stream
          </button>
          <button 
            class="tab-btn ${this.activeTab === 'services' ? 'active' : ''}" 
            @click=${() => this.activeTab = 'services'}>
            Music Services (${this.services.length})
          </button>
        </div>

        ${this.activeTab === 'favorites' ? html`
          <div class="search-bar-container">
            <input 
              type="text" 
              class="search-input" 
              placeholder="Search favorites, playlists, albums, radio stations..."
              .value=${this.searchQuery}
              @input=${this._onSearchChange}
            />
            <span class="search-count">
              ${this.searchQuery ? `Showing ${filteredFavorites.length} of ${this.favorites.length}` : `${this.favorites.length} total`}
            </span>
          </div>

          ${filteredFavorites.length === 0 ? html`
            <div class="empty-state">
              ${this.searchQuery ? `No favorites found matching "${this.searchQuery}"` : 'No favorites pinned on this speaker yet. Pin playlists or radio stations in the Sonos app to access them here.'}
            </div>
          ` : html`
            <div class="favorites-grid">
              ${filteredFavorites.map(fav => {
                const artUrl = fav.album_art_uri ? homectlApi.getArtUrl(this.selectedSpeakerIp, fav.album_art_uri) : '';
                return html`
                  <div class="fav-card" @click=${() => this._playFavorite(fav)}>
                    <div class="fav-art-wrap">
                      ${artUrl ? html`
                        <img class="fav-art" src=${artUrl} alt=${fav.title} loading="lazy" />
                      ` : html`
                        <span class="fav-art-placeholder">🎵</span>
                      `}
                    </div>
                    <div class="fav-title" title=${fav.title}>${fav.title}</div>
                    <div class="fav-desc">${fav.description || 'Sonos Item'}</div>
                    <button class="fav-play-btn" @click=${(e: Event) => { e.stopPropagation(); this._playFavorite(fav); }}>
                      <span>▶</span> Play
                    </button>
                  </div>
                `;
              })}
            </div>
          `}
        ` : ''}

        ${this.activeTab === 'stream' ? html`
          <div>
            <div class="form-group">
              <label class="form-label" for="stream-url">Audio Stream URL (HTTP/HTTPS):</label>
              <input 
                id="stream-url" 
                type="url" 
                class="text-input" 
                placeholder="https://stream.somafm.com/groovesalad-128-mp3"
                .value=${this.streamUrl}
                @input=${(e: any) => this.streamUrl = e.target.value}
              />
            </div>

            <div class="form-group">
              <label class="form-label" for="stream-title">Display Title (optional):</label>
              <input 
                id="stream-title" 
                type="text" 
                class="text-input" 
                placeholder="e.g. SomaFM Groove Salad"
                .value=${this.streamTitle}
                @input=${(e: any) => this.streamTitle = e.target.value}
              />
            </div>

            <div class="btn-group">
              <button class="btn-primary" @click=${this._playStream} ?disabled=${!this.streamUrl}>
                ▶ Play Stream Now
              </button>
              <button class="btn-secondary" @click=${this._queueAdd} ?disabled=${!this.streamUrl}>
                + Add as Next in Queue
              </button>
            </div>
          </div>
        ` : ''}

        ${this.activeTab === 'services' ? html`
          <div>
            ${this.services.length === 0 ? html`
              <div class="empty-state">No music services reported by this speaker.</div>
            ` : html`
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
                  ${this.services.map(s => html`
                    <tr>
                      <td style="font-weight: 600;">${s.name}</td>
                      <td style="color: #64748b;">${s.id}</td>
                      <td>
                        ${s.is_default ? html`<span class="default-tag">DEFAULT SERVICE</span>` : ''}
                      </td>
                      <td style="color: #64748b;">${s.version || '1.1'}</td>
                    </tr>
                  `)}
                </tbody>
              </table>

              <div class="service-info-callout">
                <strong>Music Service Integration:</strong> Sonos Favorites provide instant, credential-free playback of pinned cloud playlists and radio across all registered providers. The marked default service is used by agents when selecting ambient music without an explicit provider.
              </div>
            `}
          </div>
        ` : ''}
      </div>
    `;
  }
}
