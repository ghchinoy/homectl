import { html, css, nothing, type PropertyValues } from 'lit';
import { customElement, property, query, state } from 'lit/decorators.js';
import '@ghchinoy/lustrous-lit/components/lustrous-badge';
import '@ghchinoy/lustrous-lit/components/lustrous-icon';
import '@ghchinoy/lustrous-lit/components/lustrous-icon-button';
import { lustrousType } from '@ghchinoy/lustrous-lit/theme';
import { BaseCard } from './base-card';
import type { CastStatus } from '../api';

@customElement('cast-card')
export class CastCard extends BaseCard {
  @property({ type: String }) name = '';
  @property({ type: String }) model = '';
  @property({ type: Object }) status?: CastStatus;

  @state() private draftVolume: number | null = null;
  @query('input.slider') private sliderEl!: HTMLInputElement | null;

  static override styles = [
    ...BaseCard.styles,
    css`
      .card {
        --_slider-accent: var(--lustrous-color-positive);
      }

      .cast-info {
        padding: var(--lustrous-space-s) var(--lustrous-space-m);
        border-radius: var(--lustrous-radius-m);
        background-color: var(--lustrous-color-surface-container-low);
        border: 1px solid var(--lustrous-color-outline-variant);
        display: flex;
        flex-direction: column;
        gap: 2px;
      }

      .app-name {
        color: var(--lustrous-color-positive);
        ${lustrousType('body-medium-emphasized')}
      }

      .status-text {
        color: var(--lustrous-text-muted);
        ${lustrousType('body-small')}
      }

      .idle-state {
        margin: 0;
        color: var(--lustrous-text-muted);
        ${lustrousType('body-small')}
      }

      .cast-controls {
        display: flex;
        justify-content: center;
        align-items: center;
        gap: var(--lustrous-space-s);
        padding-top: var(--lustrous-space-s);
        border-top: 1px solid var(--lustrous-color-outline-variant);
      }
    `,
  ];

  private get effectiveVolume(): number {
    return this.draftVolume ?? this.status?.volume ?? 0;
  }

  protected override updated(changed: PropertyValues): void {
    super.updated(changed);
    this.syncSliderProgress(this.sliderEl, this.effectiveVolume);
  }

  private _onControl(action: string) {
    this.dispatchEvent(
      new CustomEvent('control-change', {
        detail: { action },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onVolumeInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const val = parseInt(input.value, 10);
    this.draftVolume = val;
    this.syncSliderProgress(input, val);
  }

  private _onVolumeChange(e: Event) {
    const input = e.target as HTMLInputElement;
    const volume = parseInt(input.value, 10);
    this.draftVolume = null;
    this.dispatchEvent(
      new CustomEvent('volume-change', {
        detail: { volume },
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected override render() {
    const volume = Math.max(0, Math.min(100, Math.round(this.effectiveVolume)));

    return html`
      <div class="card">
        <div class="card-header">
          <div class="badge-row">
            <lustrous-badge tone="emerald" icon="devices">Google Cast</lustrous-badge>
            ${this.status?.display_name ? html`<lustrous-badge tone="positive">Active</lustrous-badge>` : nothing}
          </div>
        </div>

        <div class="card-title-group">
          <h3 class="card-title">${this.name}</h3>
          ${this.model ? html`<p class="card-subtitle">${this.model}</p>` : nothing}
        </div>

        ${this.status && this.status.display_name
          ? html`
              <div class="cast-info">
                <div class="app-name">${this.status.display_name}</div>
                ${this.status.status_text ? html`<div class="status-text">${this.status.status_text}</div>` : nothing}
              </div>
            `
          : html`<p class="idle-state">Ready to Cast</p>`}

        <div class="cast-controls">
          <lustrous-icon-button
            icon="stop"
            variant="on-surface-variant"
            size="m"
            aria-label="Stop casting on ${this.name}"
            @click=${() => this._onControl('stop')}
          ></lustrous-icon-button>
          <lustrous-icon-button
            icon="playArrow"
            variant="tonal"
            size="m"
            aria-label="Play on ${this.name}"
            @click=${() => this._onControl('play')}
          ></lustrous-icon-button>
          <lustrous-icon-button
            icon="pause"
            variant="on-surface-variant"
            size="m"
            aria-label="Pause on ${this.name}"
            @click=${() => this._onControl('pause')}
          ></lustrous-icon-button>
        </div>

        <div class="control-row">
          <lustrous-icon name=${volume === 0 ? 'volumeOff' : 'volumeUp'} size="s" muted></lustrous-icon>
          <input
            type="range"
            class="slider"
            min="0"
            max="100"
            aria-label="${this.name} volume"
            aria-valuetext="${volume}%"
            .value=${String(volume)}
            @input=${this._onVolumeInput}
            @change=${this._onVolumeChange}
          />
          <span class="value-readout">${volume}%</span>
        </div>
      </div>
    `;
  }
}
