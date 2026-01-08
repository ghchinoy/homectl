import { html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { BaseCard } from './base-card';
import type { CastStatus } from '../api';

@customElement('cast-card')
export class CastCard extends BaseCard {
  @property({ type: String }) name = '';
  @property({ type: String }) model = '';
  @property({ type: Object }) status?: CastStatus;

  static styles = [
    ...BaseCard.styles,
    css`
      .badge-cast { background: #e8f5e9; color: #2e7d32; }
      .cast-info { margin-top: 10px; }
      .app-name { font-weight: bold; color: #2e7d32; font-size: 0.9rem; }
      .status-text { font-size: 0.8rem; color: #7f8c8d; font-style: italic; }
      .slider-cast::-webkit-slider-thumb { background: #2e7d32; }

      .cast-controls {
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
    return html`
      <div class="card">
        <span class="badge badge-cast">Google Cast</span>
        <h3>${this.name}</h3>
        <p style="font-size: 0.8rem; color: #7f8c8d;">${this.model}</p>
        
        ${this.status && this.status.display_name ? html`
          <div class="cast-info">
            <div class="app-name">${this.status.display_name}</div>
            ${this.status.status_text ? html`<div class="status-text">${this.status.status_text}</div>` : ''}
          </div>
        ` : html`<p style="font-size: 0.8rem; color: #bdc3c7;">Ready to Cast</p>`}

        <div class="cast-controls">
          <button class="btn-icon" @click=${() => this._onControl('stop')}>⏹</button>
          <button class="btn-icon" style="font-size: 1.5rem;" @click=${() => this._onControl('play')}>▶</button>
          <button class="btn-icon" style="font-size: 1.5rem;" @click=${() => this._onControl('pause')}>⏸</button>
        </div>

        <div class="control-row">
          <span style="font-size: 1.2rem;">🔈</span>
          <input type="range" class="slider slider-cast" min="0" max="100" 
            .value=${(this.status?.volume ?? 0).toString()} 
            @change=${this._onVolumeChange}
          >
          <span style="font-size: 0.85rem; font-weight: bold; min-width: 35px; text-align: right;">
            ${Math.round(this.status?.volume ?? 0)}%
          </span>
        </div>
      </div>
    `;
  }
}