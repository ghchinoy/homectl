import { html, css, nothing, type PropertyValues } from 'lit';
import { customElement, property, query, state } from 'lit/decorators.js';
import '@ghchinoy/lustrous-lit/components/lustrous-badge';
import '@ghchinoy/lustrous-lit/components/lustrous-icon';
import '@ghchinoy/lustrous-lit/components/lustrous-icon-button';
import { lustrousType } from '@ghchinoy/lustrous-lit/theme';
import { BaseCard } from './base-card';
import type { SonosStatus } from '../api';

@customElement('sonos-card')
export class SonosCard extends BaseCard {
  @property({ type: String }) name = '';
  @property({ type: Object }) status?: SonosStatus;
  @property({ type: String }) artUrl = '';

  @state() private draftVolume: number | null = null;
  @state() private artFailed = false;
  @query('input.slider') private sliderEl!: HTMLInputElement | null;

  static override styles = [
    ...BaseCard.styles,
    css`
      .card {
        --_slider-accent: var(--lustrous-color-warning);
      }

      .sonos-info {
        display: flex;
        align-items: center;
        gap: var(--lustrous-space-m);
        padding: var(--lustrous-space-s);
        border-radius: var(--lustrous-radius-m);
        background-color: var(--lustrous-color-surface-container-low);
        border: 1px solid var(--lustrous-color-outline-variant);
      }

      .sonos-art {
        width: 64px;
        height: 64px;
        flex-shrink: 0;
        border-radius: var(--lustrous-radius-s);
        background-color: var(--lustrous-color-surface-container);
        object-fit: cover;
        display: flex;
        align-items: center;
        justify-content: center;
        overflow: hidden;
      }

      .sonos-art img {
        width: 100%;
        height: 100%;
        object-fit: cover;
        display: block;
      }

      .track-meta {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
        gap: 2px;
      }

      .track-title {
        color: var(--lustrous-color-on-surface);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        ${lustrousType('body-medium-emphasized')}
      }

      .track-artist {
        color: var(--lustrous-text-muted);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        ${lustrousType('body-small')}
      }

      .idle-state {
        margin: 0;
        color: var(--lustrous-text-muted);
        ${lustrousType('body-small')}
      }

      .sonos-controls {
        display: flex;
        justify-content: center;
        align-items: center;
        gap: var(--lustrous-space-s);
        padding-top: var(--lustrous-space-s);
        border-top: 1px solid var(--lustrous-color-outline-variant);
      }
    `,
  ];

  protected override willUpdate(changed: PropertyValues): void {
    super.willUpdate(changed);
    if (changed.has('artUrl')) {
      this.artFailed = false;
    }
  }

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
    const isPlaying = this.status?.status === 'PLAYING';
    const volume = Math.max(0, Math.min(100, Math.round(this.effectiveVolume)));

    return html`
      <div class="card">
        <div class="card-header">
          <div class="badge-row">
            <lustrous-badge tone="amber" icon="headset">Sonos</lustrous-badge>
            ${isPlaying ? html`<lustrous-badge tone="positive">Playing</lustrous-badge>` : nothing}
          </div>
        </div>

        <div class="card-title-group">
          <h3 class="card-title">${this.name}</h3>
        </div>

        ${this.status && this.status.title
          ? html`
              <div class="sonos-info">
                <div class="sonos-art">
                  ${this.artUrl && !this.artFailed
                    ? html`<img
                        src=${this.artUrl}
                        alt="${this.status.title} album art"
                        loading="lazy"
                        @error=${() => {
                          this.artFailed = true;
                        }}
                      />`
                    : html`<lustrous-icon name="headset" size="m" muted></lustrous-icon>`}
                </div>
                <div class="track-meta">
                  <div class="track-title" title=${this.status.title}>${this.status.title}</div>
                  ${this.status.artist
                    ? html`<div class="track-artist" title=${this.status.artist}>${this.status.artist}</div>`
                    : nothing}
                </div>
              </div>
            `
          : html`<p class="idle-state">Nothing playing</p>`}

        <div class="sonos-controls">
          <lustrous-icon-button
            icon="skipPrevious"
            variant="on-surface-variant"
            size="m"
            aria-label="Previous track on ${this.name}"
            @click=${() => this._onControl('prev')}
          ></lustrous-icon-button>
          <lustrous-icon-button
            icon=${isPlaying ? 'pause' : 'playArrow'}
            variant="tonal"
            size="m"
            aria-label="${isPlaying ? 'Pause' : 'Play'} on ${this.name}"
            @click=${() => this._onControl(isPlaying ? 'pause' : 'play')}
          ></lustrous-icon-button>
          <lustrous-icon-button
            icon="skipNext"
            variant="on-surface-variant"
            size="m"
            aria-label="Next track on ${this.name}"
            @click=${() => this._onControl('next')}
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
