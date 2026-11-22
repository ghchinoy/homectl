import { html, css, nothing } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import '@ghchinoy/lustrous-lit/components/lustrous-badge';
import '@ghchinoy/lustrous-lit/components/lustrous-button';
import '@ghchinoy/lustrous-lit/components/lustrous-progress-bar';
import { lustrousType } from '@ghchinoy/lustrous-lit/theme';
import { BaseCard } from './base-card';
import type { RoborockDevice } from '../api';

@customElement('roborock-card')
export class RoborockCard extends BaseCard {
  @property({ type: Object }) device?: RoborockDevice;

  static override styles = [
    ...BaseCard.styles,
    css`
      .meta-row {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--lustrous-space-xs);
        color: var(--lustrous-text-muted);
        ${lustrousType('body-small')}
      }

      .meta-strong {
        color: var(--lustrous-color-on-surface);
        ${lustrousType('body-small-emphasized')}
      }

      .battery-section {
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-xs);
      }

      .battery-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        color: var(--lustrous-text-muted);
        ${lustrousType('caption')}
      }

      .rr-controls {
        margin-top: auto;
        padding-top: var(--lustrous-space-m);
        border-top: 1px solid var(--lustrous-color-outline-variant);
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: var(--lustrous-space-s);
      }

      .rr-controls lustrous-button {
        width: 100%;
      }
    `,
  ];

  private _onAction(action: string) {
    if (!this.device) return;
    this.dispatchEvent(
      new CustomEvent('roborock-control', {
        detail: { id: this.device.did || this.device.duid, action },
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected override render() {
    const d = this.device;
    if (!d) return nothing;

    const battery = d.battery !== undefined ? Math.max(0, Math.min(100, Math.round(d.battery))) : undefined;
    const batteryTone =
      battery === undefined ? 'default' : battery <= 20 ? 'error' : battery <= 40 ? 'warning' : 'positive';
    const hasError = (d.error_code ?? 0) > 0;
    const statusLabel = d.state_name || (d.connected ? 'ready' : d.key_recovery_state || 'onboarding');

    return html`
      <div class="card">
        <div class="card-header">
          <div class="badge-row">
            <lustrous-badge tone="amber" icon="bolt">Roborock</lustrous-badge>
            <lustrous-badge tone=${d.connected ? 'positive' : 'default'}>
              ${d.connected ? 'Online' : 'Offline'}
            </lustrous-badge>
            ${hasError
              ? html`<lustrous-badge tone="error" icon="warning">${d.error_name || 'Error'}</lustrous-badge>`
              : nothing}
          </div>
        </div>

        <div class="card-title-group">
          <h3 class="card-title">${d.name || d.model || d.did}</h3>
          <p class="card-subtitle">
            ${d.model || 'Unknown model'}${d.firmware_version ? ` · fw ${d.firmware_version}` : ''}
          </p>
        </div>

        <div class="meta-row">
          <span>State:</span>
          <span class="meta-strong">${statusLabel}</span>
        </div>

        ${battery !== undefined
          ? html`
              <div class="battery-section">
                <div class="battery-header">
                  <span>Battery</span>
                  <span class="meta-strong">${battery}%</span>
                </div>
                <lustrous-progress-bar
                  .value=${battery}
                  tone=${batteryTone}
                  label="${d.name || d.model || 'Roborock'} battery"
                  decorative
                ></lustrous-progress-bar>
              </div>
            `
          : nothing}

        <div class="rr-controls">
          <lustrous-button
            variant="tonal"
            size="s"
            icon="playArrow"
            ?disabled=${!d.connected}
            @click=${() => this._onAction('start')}
          >
            Clean
          </lustrous-button>
          <lustrous-button
            variant="outline"
            size="s"
            icon="pause"
            ?disabled=${!d.connected}
            @click=${() => this._onAction('pause')}
          >
            Pause
          </lustrous-button>
          <lustrous-button
            variant="outline"
            size="s"
            icon="home"
            ?disabled=${!d.connected}
            @click=${() => this._onAction('dock')}
          >
            Dock
          </lustrous-button>
          <lustrous-button
            variant="outline"
            size="s"
            icon="locationOn"
            ?disabled=${!d.connected}
            @click=${() => this._onAction('find')}
          >
            Find
          </lustrous-button>
        </div>
      </div>
    `;
  }
}
