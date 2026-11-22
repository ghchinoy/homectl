import { html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { BaseCard } from './base-card';
import type { RoborockDevice } from '../api';

@customElement('roborock-card')
export class RoborockCard extends BaseCard {
  @property({ type: Object }) device?: RoborockDevice;

  static styles = [
    ...BaseCard.styles,
    css`
      .badge-roborock { background: #fff3e0; color: #e65100; }
      .badge-online { background: #e8f5e9; color: #2e7d32; margin-left: 6px; }
      .badge-offline { background: #eceff1; color: #546e7a; margin-left: 6px; }
      .badge-error { background: #ffebee; color: #c62828; margin-left: 6px; }
      .meta { font-size: 0.8rem; color: #7f8c8d; margin: 4px 0; }
      .battery-bar {
        height: 6px;
        background: #ecf0f1;
        border-radius: 3px;
        overflow: hidden;
        margin: 8px 0;
      }
      .battery-fill {
        height: 100%;
        background: #e65100;
        transition: width 0.3s ease;
      }
      .rr-controls {
        display: flex;
        justify-content: space-between;
        gap: 8px;
        margin-top: 14px;
        padding-top: 12px;
        border-top: 1px solid #f1f2f6;
      }
      .rr-btn {
        flex: 1;
        padding: 6px 8px;
        border: 1px solid #dee2e6;
        border-radius: 6px;
        background: #fff;
        color: #2c3e50;
        font-size: 0.8rem;
        cursor: pointer;
        transition: background 0.15s;
      }
      .rr-btn:hover { background: #f8f9fa; }
      .rr-btn:disabled { opacity: 0.45; cursor: not-allowed; }
    `
  ];

  private _onAction(action: string) {
    if (!this.device) return;
    this.dispatchEvent(new CustomEvent('roborock-control', {
      detail: { id: this.device.did || this.device.duid, action },
      bubbles: true,
      composed: true
    }));
  }

  render() {
    const d = this.device;
    if (!d) return html``;
    const battery = d.battery ?? 0;
    const hasError = (d.error_code ?? 0) > 0;

    return html`
      <div class="card">
        <div>
          <span class="badge badge-roborock">Roborock</span>
          <span class="badge ${d.connected ? 'badge-online' : 'badge-offline'}">
            ${d.connected ? 'Online' : 'Offline'}
          </span>
          ${hasError ? html`<span class="badge badge-error">${d.error_name || 'Error'}</span>` : ''}
        </div>

        <h3>${d.name || d.model || d.did}</h3>
        <div class="meta">
          ${d.model || 'Unknown model'}
          ${d.firmware_version ? ` · fw ${d.firmware_version}` : ''}
        </div>
        <div class="meta">
          State: <strong>${d.state_name || (d.connected ? 'ready' : d.key_recovery_state || 'onboarding')}</strong>
          ${d.battery !== undefined ? ` · Battery: ${d.battery}%` : ''}
        </div>

        ${d.battery !== undefined ? html`
          <div class="battery-bar">
            <div class="battery-fill" style="width: ${Math.max(0, Math.min(100, battery))}%"></div>
          </div>
        ` : ''}

        <div class="rr-controls">
          <button class="rr-btn" ?disabled=${!d.connected} @click=${() => this._onAction('start')}>Clean</button>
          <button class="rr-btn" ?disabled=${!d.connected} @click=${() => this._onAction('pause')}>Pause</button>
          <button class="rr-btn" ?disabled=${!d.connected} @click=${() => this._onAction('dock')}>Dock</button>
          <button class="rr-btn" ?disabled=${!d.connected} @click=${() => this._onAction('find')}>Find</button>
        </div>
      </div>
    `;
  }
}
