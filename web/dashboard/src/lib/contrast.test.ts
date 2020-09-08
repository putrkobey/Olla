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
