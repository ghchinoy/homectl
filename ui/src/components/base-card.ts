import { css } from 'lit';
import {
  LustrousElement,
  lustrousBase,
  lustrousFocusRing,
  lustrousStateLayer,
  lustrousType,
} from '@ghchinoy/lustrous-lit/theme';

export class BaseCard extends LustrousElement {
  static override styles = [
    lustrousBase,
    lustrousFocusRing,
    lustrousStateLayer,
    css`
      :host {
        display: block;
        height: 100%;
        container-type: inline-size;
      }

      .card {
        background-color: var(--lustrous-color-surface);
        color: var(--lustrous-color-on-surface);
        border: 1px solid var(--lustrous-color-outline-variant);
        border-radius: var(--lustrous-radius-l);
        padding: var(--lustrous-space-xl);
        box-shadow: var(--lustrous-elevation-level1);
        transition:
          transform var(--lustrous-duration-short200) var(--lustrous-easing-standard),
          box-shadow var(--lustrous-duration-short200) var(--lustrous-easing-standard),
          border-color var(--lustrous-duration-short200) var(--lustrous-easing-standard);
        height: 100%;
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-m);
      }

      .card:hover {
        transform: translateY(-2px);
        box-shadow: var(--lustrous-elevation-level2);
      }

      .card-header {
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        gap: var(--lustrous-space-s);
      }

      .badge-row {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--lustrous-space-xs);
      }

      .card-title-group {
        display: flex;
        flex-direction: column;
        gap: var(--lustrous-space-xs);
        min-width: 0;
      }

      .card-title {
        margin: 0;
        color: inherit;
        overflow-wrap: anywhere;
        ${lustrousType('headline-small')}
      }

      .card-subtitle {
        margin: 0;
        color: var(--lustrous-text-muted);
        ${lustrousType('body-small')}
      }

      .control-row {
        margin-top: auto;
        padding-top: var(--lustrous-space-s);
        display: flex;
        align-items: center;
        gap: var(--lustrous-space-m);
      }

      .value-readout {
        min-width: 40px;
        text-align: end;
        font-variant-numeric: tabular-nums;
        color: var(--lustrous-color-on-surface);
        ${lustrousType('body-small-emphasized')}
      }

      /* Accessible native range slider styled with Lustrous tokens.
         WebKit and Firefox pseudo-elements are intentionally kept in separate blocks. */
      .slider {
        --_slider-track-size: 6px;
        --_slider-thumb-size: 20px;
        --_slider-track-color: var(--lustrous-color-surface-container-highest);
        --_slider-fill-color: var(--_slider-accent, var(--lustrous-color-primary));
        --_slider-direction: to right;
        appearance: none;
        -webkit-appearance: none;
        flex: 1;
        min-width: 0;
        inline-size: 100%;
        block-size: 40px;
        margin: 0;
        padding: 0;
        border: 0;
        border-radius: 0;
        background: transparent;
        cursor: pointer;
        font: inherit;
      }

      .slider:dir(rtl) {
        --_slider-direction: to left;
      }

      .slider:focus-visible {
        outline: 2.5px solid var(--_slider-fill-color);
        outline-offset: 2px;
        border-radius: var(--lustrous-radius-s);
      }

      .slider::-webkit-slider-runnable-track {
        block-size: var(--_slider-track-size);
        border: 0;
        border-radius: var(--lustrous-radius-full);
        background: linear-gradient(
          var(--_slider-direction),
          var(--_slider-fill-color) var(--_slider-progress, 0%),
          var(--_slider-track-color) var(--_slider-progress, 0%)
        );
      }

      .slider::-webkit-slider-thumb {
        -webkit-appearance: none;
        appearance: none;
        inline-size: var(--_slider-thumb-size);
        block-size: var(--_slider-thumb-size);
        margin-block-start: calc((var(--_slider-track-size) - var(--_slider-thumb-size)) / 2);
        border: 2px solid var(--lustrous-color-surface);
        border-radius: var(--lustrous-radius-full);
        background: var(--_slider-fill-color);
        box-shadow: var(--lustrous-elevation-level1);
        transition: transform var(--lustrous-duration-short100) var(--lustrous-easing-standard);
      }

      .slider:hover::-webkit-slider-thumb {
        transform: scale(1.08);
      }

      .slider::-moz-range-track {
        block-size: var(--_slider-track-size);
        border: 0;
        border-radius: var(--lustrous-radius-full);
        background: var(--_slider-track-color);
      }

      .slider::-moz-range-progress {
        block-size: var(--_slider-track-size);
        border-radius: var(--lustrous-radius-full);
        background: var(--_slider-fill-color);
      }

      .slider::-moz-range-thumb {
        inline-size: var(--_slider-thumb-size);
        block-size: var(--_slider-thumb-size);
        border: 2px solid var(--lustrous-color-surface);
        border-radius: var(--lustrous-radius-full);
        background: var(--_slider-fill-color);
        box-shadow: var(--lustrous-elevation-level1);
      }

      @media (forced-colors: active) {
        .card {
          border: 1px solid CanvasText;
          box-shadow: none;
        }
        .slider:focus-visible {
          outline-color: Highlight;
        }
        .slider::-webkit-slider-runnable-track {
          border: 1px solid ButtonBorder;
          background: Field;
        }
        .slider::-webkit-slider-thumb {
          background: Highlight;
          border-color: Canvas;
        }
        .slider::-moz-range-track {
          border: 1px solid ButtonBorder;
          background: Field;
        }
        .slider::-moz-range-progress {
          background: Highlight;
        }
        .slider::-moz-range-thumb {
          background: Highlight;
          border-color: Canvas;
        }
      }
    `,
  ];

  protected syncSliderProgress(input: HTMLInputElement | null, value: number, min = 0, max = 100): void {
    if (!input) return;
    const clamped = Math.max(min, Math.min(max, Number.isFinite(value) ? value : min));
    const pct = max > min ? ((clamped - min) / (max - min)) * 100 : 0;
    input.style.setProperty('--_slider-progress', `${pct}%`);
  }
}
