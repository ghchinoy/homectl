import { html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { BaseCard } from './base-card';
import type { SonosStatus } from '../api';

@customElement('sonos-card')
export class SonosCard extends BaseCard {
  @property({ type: String }) name = '';
  @property({ type: Object }) status?: SonosStatus;
  @property({ type: String }) artUrl = '';

  static styles = [
    ...BaseCard.styles,
    css`
      .badge-sonos { background: #fff3e0; color: #e65100; }
      .sonos-art {
        width: 80px;
        height: 80px;
        border-radius: 6px;
        background: #eee;
        object-fit: cover;
        margin-right: 15px;
      }
      .sonos-info { display: flex; align-items: center; margin-top: 10px; }
      .track-meta { flex: 1; min-width: 0; }
      .track-title { font-weight: bold; font-size: 0.95rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
      .track-artist { color: #7f8c8d; font-size: 0.85rem; }

      .sonos-controls {
        display: flex;
        justify-content: center;
        gap: 15px;
        margin-top: 15px;
        padding-top: 15px;
        border-top: 1px solid #f1f2f6;
      }
      .btn-icon {
        background: none;
        border: none;
        font-size: 1.2rem;
        cursor: pointer;
        color: #2c3e50;
        padding: 5px;
        border-radius: 50%;
        width: 40px;
        height: 40px;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: background 0.2s;
      }
      .btn-icon:hover { background: #f1f2f6; }
      .slider-sonos::-webkit-slider-thumb { background: #e67e22; }
    `
  ];

  private _onControl(action: string) {
    this.dispatchEvent(new CustomEvent('control-change', {
      detail: { action },
      bubbles: true,
      composed: true
    }));
  }

  private _onVolumeChange(e: any) {
    const volume = parseInt(e.target.value);
    this.dispatchEvent(new CustomEvent('volume-change', {
      detail: { volume },
      bubbles: true,
      composed: true
    }));
  }

  render() {
    const isPlaying = this.status?.status === 'PLAYING';
    
    return html`
      <div class="card">
        <span class="badge badge-sonos">Sonos</span>
        <h3>${this.name}</h3>
        
        ${this.status && this.status.title ? html`
          <div class="sonos-info">
            ${this.artUrl ? html`
              <img class="sonos-art" src=${this.artUrl} />
            ` : html`<div class="sonos-art"></div>`}
            <div class="track-meta">
              <div class="track-title">${this.status.title}</div>
              <div class="track-artist">${this.status.artist}</div>
            </div>
          </div>
        ` : html`<p>Nothing playing</p>`}

        <div class="sonos-controls">
          <button class="btn-icon" @click=${() => this._onControl('prev')}>⏮</button>
          <button class="btn-icon" style="font-size: 1.5rem;" @click=${() => this._onControl(isPlaying ? 'pause' : 'play')}>
            ${isPlaying ? '⏸' : '▶'}
          </button>
          <button class="btn-icon" @click=${() => this._onControl('next')}>⏭</button>
        </div>

        <div class="control-row">
          <span style="font-size: 1.2rem;">🔈</span>
          <input type="range" class="slider slider-sonos" min="0" max="100" 
            .value=${(this.status?.volume ?? 0).toString()} 
            @change=${this._onVolumeChange}
          >
          <span style="font-size: 0.85rem; font-weight: bold; min-width: 35px; text-align: right;">
            ${this.status?.volume ?? 0}%
          </span>
        </div>
      </div>
    `;
  }
}
