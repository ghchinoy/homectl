import { html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { BaseCard } from './base-card';

@customElement('camera-card')
export class CameraCard extends BaseCard {
  @property({ type: String }) name = '';
  @property({ type: String }) ip = '';
  @state() private streaming = false;

  static styles = [
    ...BaseCard.styles,
    css`
      .badge-security { background: #fce4ec; color: #880e4f; }
      .camera-box {
        width: 100%;
        aspect-ratio: 16/9;
        background: #2c3e50;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        color: #ecf0f1;
        font-size: 0.8rem;
        margin-top: 10px;
        overflow: hidden;
      }
      .camera-stream {
        width: 100%;
        height: 100%;
        object-fit: cover;
      }
      .btn-stream {
        margin-top: 10px;
        background: #3498db;
        color: white;
        border: none;
        padding: 8px 15px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.8rem;
        width: 100%;
      }
      .btn-stream.active { background: #e74c3c; }
      .vlc-link {
        display: block;
        text-align: center;
        margin-top: 10px;
        color: #3498db;
        text-decoration: none;
        font-size: 0.75rem;
      }
    `
  ];

  private _toggleStream() {
    this.streaming = !this.streaming;
  }

  render() {
    return html`
      <div class="card">
        <span class="badge badge-security">Security</span>
        <h3>${this.name}</h3>
        <div class="camera-box">
          ${this.streaming 
            ? html`<img class="camera-stream" src="/api/security/stream?ip=${this.ip}" />`
            : html`<div>Stream Offline</div>`
          }
        </div>
        <button 
          class="btn-stream ${this.streaming ? 'active' : ''}" 
          @click=${this._toggleStream}>
          ${this.streaming ? 'Stop Stream' : 'Enable Live Stream'}
        </button>
        <a class="vlc-link" href="rtsp://${this.ip}:554">Open RTSP Direct</a>
      </div>
    `;
  }
}
