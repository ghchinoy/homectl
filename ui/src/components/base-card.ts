import { LitElement, css } from 'lit';

export class BaseCard extends LitElement {
  static styles = [css`
    .card {
      background: white;
      border-radius: 12px;
      padding: 20px;
      box-shadow: 0 4px 6px rgba(0,0,0,0.05);
      transition: transform 0.2s, background-color 0.3s, color 0.3s;
      height: 100%;
      box-sizing: border-box;
      display: flex;
      flex-direction: column;
    }
    .card:hover { transform: translateY(-2px); }

    .badge {
      display: inline-block;
      padding: 4px 10px;
      border-radius: 20px;
      font-size: 0.7rem;
      font-weight: 700;
      text-transform: uppercase;
      margin-bottom: 12px;
      width: fit-content;
    }

    h3 {
      margin: 0 0 10px 0;
      font-size: 1.1rem;
      color: inherit;
    }
    p {
      margin: 5px 0;
      font-size: 0.9rem;
      color: inherit;
    }

    .control-row {
      margin-top: auto;
      padding-top: 15px;
      display: flex;
      align-items: center;
      gap: 10px;
    }

    .slider {
      flex: 1;
      height: 6px;
      background: #dfe6e9;
      border-radius: 3px;
      outline: none;
      -webkit-appearance: none;
    }
    .slider::-webkit-slider-thumb {
      -webkit-appearance: none;
      width: 18px;
      height: 18px;
      background: #0984e3;
      border-radius: 50%;
      cursor: pointer;
    }
  `];
}
