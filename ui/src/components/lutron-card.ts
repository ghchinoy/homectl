import { html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { styleMap } from 'lit/directives/style-map.js';
import { BaseCard } from './base-card';

@customElement('lutron-card')
export class LutronCard extends BaseCard {
  @property({ type: String }) name = '';
  @property({ type: String }) subtitle = '';
  @property({ type: Number }) level = 0;
  @property({ type: Boolean }) isMaster = false;

  static styles = [
    ...BaseCard.styles,
    css`
      .badge-lutron { background: rgba(13, 71, 161, 0.1); color: inherit; border: 1px solid currentColor; }
      .btn-off {
        background: #e74c3c;
        color: white;
        border: none;
        padding: 5px 12px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.7rem;
        font-weight: bold;
        margin-left: auto;
      }
      .level-label {
        font-size: 0.85rem;
        font-weight: bold;
        min-width: 35px;
        text-align: right;
      }
    `
  ];

  private _getDynamicStyle() {
    const ratio = this.level / 100;
    const r = Math.round(44 + (255 - 44) * ratio);
    const g = Math.round(62 + (255 - 62) * ratio);
    const b = Math.round(80 + (255 - 80) * ratio);
    
    const backgroundColor = `rgb(${r}, ${g}, ${b})`;
    const color = this.level < 50 ? '#ffffff' : '#2c3e50';
    
    return {
      backgroundColor: backgroundColor,
      color: color,
      boxShadow: this.level > 0 ? `0 0 ${this.level/4}px rgba(255, 255, 200, ${ratio * 0.5})` : 'none',
      border: this.level < 10 ? '1px solid #34495e' : '1px solid #eee'
    };
  }

  private _onLevelChange(e: any) {
    const newLevel = parseInt(e.target.value);
    this.dispatchEvent(new CustomEvent('level-change', {
      detail: { level: newLevel },
      bubbles: true,
      composed: true
    }));
  }

  private _onOffClick() {
    this.dispatchEvent(new CustomEvent('level-change', {
      detail: { level: 0 },
      bubbles: true,
      composed: true
    }));
  }

  render() {
    return html`
      <div class="card" style=${styleMap(this._getDynamicStyle())}>
        <div style="display: flex; align-items: flex-start;">
          <div>
            <span class="badge badge-lutron">Lutron</span>
            <h3>${this.name}</h3>
            <p style="opacity: 0.8;">${this.subtitle}</p>
          </div>
          ${this.isMaster ? html`<button class="btn-off" @click=${this._onOffClick}>ALL OFF</button>` : ''}
        </div>
        
        <div class="control-row">
          <input type="range" class="slider" min="0" max="100" 
            .value=${this.level.toString()} 
            @change=${this._onLevelChange}
          >
          <span class="level-label">${Math.round(this.level)}%</span>
        </div>
      </div>
    `;
  }
}
