import { html, css, nothing, type PropertyValues } from 'lit';
import { customElement, property, query, state } from 'lit/decorators.js';
import '@ghchinoy/lustrous-lit/components/lustrous-badge';
import '@ghchinoy/lustrous-lit/components/lustrous-button';
import '@ghchinoy/lustrous-lit/components/lustrous-icon';
import { BaseCard } from './base-card';

@customElement('lutron-card')
export class LutronCard extends BaseCard {
  @property({ type: String }) name = '';
  @property({ type: String }) subtitle = '';
  @property({ type: Number }) level = 0;
  @property({ type: Boolean }) isMaster = false;

  @state() private draftLevel: number | null = null;
  @query('input.slider') private sliderEl!: HTMLInputElement | null;

  static override styles = [
    ...BaseCard.styles,
    css`
      .card {
        position: relative;
        overflow: hidden;
        --_slider-accent: var(--lustrous-color-primary);
      }

      .card.is-lit {
        border-color: color-mix(
          in srgb,
          var(--lustrous-color-warning) calc(var(--_light-pct, 0) * 0.45%),
          var(--lustrous-color-outline-variant)
        );
        background-color: color-mix(
          in srgb,
          var(--lustrous-color-warning-container) calc(var(--_light-pct, 0) * 0.35%),
          var(--lustrous-color-surface)
        );
      }

      .glow-strip {
        position: absolute;
        inset-block-start: 0;
        inset-inline-start: 0;
        block-size: 3px;
        inline-size: var(--_light-width, 0%);
        background: var(--lustrous-color-warning);
        transition: inline-size var(--lustrous-duration-short200) var(--lustrous-easing-standard);
        pointer-events: none;
      }

      @media (forced-colors: active) {
        .glow-strip {
          background: Highlight;
        }
      }
    `,
  ];

  private get effectiveLevel(): number {
    return this.draftLevel ?? this.level;
  }

  protected override updated(changed: PropertyValues): void {
    super.updated(changed);
    const pct = Math.max(0, Math.min(100, Math.round(this.effectiveLevel)));
    if (this.style) {
      this.style.setProperty('--_light-pct', String(pct));
      this.style.setProperty('--_light-width', `${pct}%`);
    }
    this.syncSliderProgress(this.sliderEl, pct);
  }

  private _onInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const val = parseInt(input.value, 10);
    this.draftLevel = val;
    this.syncSliderProgress(input, val);
  }

  private _onLevelChange(e: Event) {
    const input = e.target as HTMLInputElement;
    const newLevel = parseInt(input.value, 10);
    this.draftLevel = null;
    this.dispatchEvent(
      new CustomEvent('level-change', {
        detail: { level: newLevel },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _onQuickToggle() {
    const targetLevel = this.isMaster ? 0 : this.effectiveLevel > 0 ? 0 : 100;
    this.draftLevel = null;
    this.dispatchEvent(
      new CustomEvent('level-change', {
        detail: { level: targetLevel },
        bubbles: true,
        composed: true,
      }),
    );
  }

  protected override render() {
    const pct = Math.max(0, Math.min(100, Math.round(this.effectiveLevel)));
    const isLit = pct > 0;

    return html`
      <div class="card ${isLit ? 'is-lit' : ''}">
        <div class="glow-strip" aria-hidden="true"></div>
        <div class="card-header">
          <div class="badge-row">
            <lustrous-badge tone="cobalt" icon="lightbulb">Lutron</lustrous-badge>
            ${this.isMaster ? html`<lustrous-badge tone="accent">Master</lustrous-badge>` : nothing}
          </div>
          <lustrous-button
            variant=${this.isMaster ? 'outline' : isLit ? 'tonal' : 'outline'}
            size="xs"
            @click=${this._onQuickToggle}
          >
            ${this.isMaster ? 'All Off' : isLit ? 'Off' : 'On'}
          </lustrous-button>
        </div>

        <div class="card-title-group">
          <h3 class="card-title">${this.name}</h3>
          ${this.subtitle ? html`<p class="card-subtitle">${this.subtitle}</p>` : nothing}
        </div>

        <div class="control-row">
          <lustrous-icon name="lightbulb" size="s" ?muted=${!isLit}></lustrous-icon>
          <input
            type="range"
            class="slider"
            min="0"
            max="100"
            aria-label="${this.name} brightness"
            aria-valuetext="${pct}%"
            .value=${String(pct)}
            @input=${this._onInput}
            @change=${this._onLevelChange}
          />
          <span class="value-readout">${pct}%</span>
        </div>
      </div>
    `;
  }
}
