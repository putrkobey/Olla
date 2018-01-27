import { describe, it, expect } from 'vitest';

// Token sets ported verbatim from src/app.css. If either changes, update the
// other to match. These are the actual colours shipped to operators.
const LIGHT: Record<string, string> = {
  bg: '#f1efe5',
  bgElevated: '#fbfaf4',
  bgInset: '#e7e4d6',
  bgHeader: '#e9e6d8',
  text: '#23261f',
  textDim: '#5c5a49',
  textFaint: '#6d6a58',
  accent: '#0d6b60',
  link: '#0d6b60',
  green: '#187338',
  greenBg: '#dcecdd',
  amber: '#8a5a00',
  amberBg: '#f1e4bf',
  red: '#a3241c',
  redBg: '#f3dcd8',
};
const DARK: Record<string, string> = {
  bg: '#0a0d0d',
  bgElevated: '#101414',
  bgInset: '#151a1a',
  bgHeader: '#0d1111',
  text: '#d9e4e1',
  textDim: '#8ba39d',
  textFaint: '#738581',
  accent: '#5fd9c6',
  link: '#7ee6d3',
  green: '#4fbd7a',
  greenBg: '#123322',
  amber: '#e0ac47',
  amberBg: '#3a2c0e',
  red: '#e5695f',
  redBg: '#3a1815',
};

function channel(hex: string): [number, number, number] {
  const h = hex.replace('#', '');
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
}

function luminance(hex: string): number {
  const [r, g, b] = channel(hex).map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function ratio(a: string, b: string): number {
  const la = luminance(a);
  const lb = luminance(b);
  const [hi, lo] = la > lb ? [la, lb] : [lb, la];
