import { html, css, nothing } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import '@ghchinoy/lustrous-lit/components/lustrous-badge';
import '@ghchinoy/lustrous-lit/components/lustrous-button';
import '@ghchinoy/lustrous-lit/components/lustrous-icon';
import '@ghchinoy/lustrous-lit/components/lustrous-spinner';
import { lustrousType } from '@ghchinoy/lustrous-lit/theme';
import { BaseCard } from './base-card';

@customElement('camera-card')
export class CameraCard extends BaseCard {
  @property({ type: String }) name = '';
  @property({ type: String }) ip = '';

  @state() private streaming = false;
  @state() private connecting = false;
  @state() private streamError = false;

  static override styles = [
    ...BaseCard.styles,
    css`
      .camera-box {
        position: relative;
        width: 100%;
        aspect-ratio: 16 / 9;
        background-color: var(--lustrous-color-code-background);
        color: var(--lustrous-color-code-primary-text);
        border-radius: var(--lustrous-radius-m);
        border: 1px solid var(--lustrous-color-outline-variant);
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--lustrous-space-s);
        overflow: hidden;
        ${lustrousType('body-small')}
      }

      .camera-stream {
        width: 100%;
        height: 100%;
        object-fit: cover;
        display: block;
      }

      .camera-overlay {
        position: absolute;
        inset: 0;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--lustrous-space-s);
        background-color: var(--lustrous-color-scrim-immersive);
        color: var(--lustrous-color-white);
      }

      .camera-actions {
        margin-top: auto;
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-xs);
      }

      .camera-actions lustrous-button {
        width: 100%;
      }
    `,
  ];

  private _toggleStream() {
    this.streaming = !this.streaming;
    this.connecting = this.streaming;
    this.streamError = false;
  }

  protected override render() {
    return html`
      <div class="card">
        <div class="card-header">
          <div class="badge-row">
            <lustrous-badge tone="rose" icon="photoCamera">Security</lustrous-badge>
            ${this.streaming && !this.streamError
              ? html`<lustrous-badge tone="positive">Live</lustrous-badge>`
              : nothing}
          </div>
        </div>

        <div class="card-title-group">
          <h3 class="card-title">${this.name}</h3>
        </div>

        <div class="camera-box" aria-busy=${this.connecting ? 'true' : 'false'}>
          ${this.streaming
            ? html`
                <img
                  class="camera-stream"
                  src="/api/security/stream?ip=${encodeURIComponent(this.ip)}"
                  alt="${this.name} live feed"
                  @load=${() => {
                    this.connecting = false;
                  }}
                  @error=${() => {
                    this.connecting = false;
                    this.streamError = true;
                  }}
                />
                ${this.connecting
                  ? html`
                      <div class="camera-overlay">
                        <lustrous-spinner variant="ring" size="m" label="Connecting to ${this.name}"></lustrous-spinner>
                        <span>Connecting&hellip;</span>
                      </div>
                    `
                  : nothing}
                ${this.streamError
                  ? html`
                      <div class="camera-overlay">
                        <lustrous-icon name="error" size="m"></lustrous-icon>
                        <span>Stream unavailable</span>
                      </div>
                    `
                  : nothing}
              `
            : html`
                <lustrous-icon name="photoCamera" size="l" muted></lustrous-icon>
                <span>Stream Offline</span>
              `}
        </div>

        <div class="camera-actions">
          <lustrous-button
            variant=${this.streaming ? 'outline' : 'accent'}
            size="s"
            icon=${this.streaming ? 'stop' : 'playArrow'}
            @click=${this._toggleStream}
          >
            ${this.streaming ? 'Stop Stream' : 'Enable Live Stream'}
          </lustrous-button>
          <lustrous-button
            variant="on-surface-variant"
            size="xs"
            trailing-icon="openInNew"
            href="rtsp://${this.ip}:554"
          >
            Open RTSP Direct
          </lustrous-button>
        </div>
      </div>
    `;
  }
}
