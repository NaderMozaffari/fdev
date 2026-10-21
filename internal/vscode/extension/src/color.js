'use strict';

// Colour helpers for the decorations, without "vscode" so they can be tested.
// The colours in the settings are CSS colours: "#58a6ff", "#58a6ff80",
// "#5af", "rgb(88, 166, 255)" or "rgba(88 166 255 / 0.5)". A name such as
// "orange" is passed on to VS Code as it is, but cannot be made fainter.

/** parseColor returns { r, g, b, a } (a from 0 to 1) or null. */
function parseColor(text) {
  if (typeof text !== 'string') return null;
  const s = text.trim().toLowerCase();
  let m = /^#([0-9a-f]{3,4})$/.exec(s);
  if (m) {
    const h = m[1].split('').map((c) => parseInt(c + c, 16));
    return { r: h[0], g: h[1], b: h[2], a: h.length === 4 ? h[3] / 255 : 1 };
  }
  m = /^#([0-9a-f]{6})([0-9a-f]{2})?$/.exec(s);
  if (m) {
    const n = parseInt(m[1], 16);
    return {
      r: (n >> 16) & 255,
      g: (n >> 8) & 255,
      b: n & 255,
      a: m[2] ? parseInt(m[2], 16) / 255 : 1,
    };
  }
  m = /^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)(?:\s*[,/]\s*([\d.]+)(%?))?\s*\)$/.exec(s);
  if (m) {
    const a = m[4] === undefined ? 1 : m[5] ? Number(m[4]) / 100 : Number(m[4]);
    return {
      r: Math.min(255, Math.round(Number(m[1]))),
      g: Math.min(255, Math.round(Number(m[2]))),
      b: Math.min(255, Math.round(Number(m[3]))),
      a: Math.min(1, Math.max(0, a)),
    };
  }
  return null;
}

const hex2 = (n) => Math.round(n).toString(16).padStart(2, '0');

/**
 * withAlpha makes `color` fainter by `alpha` (0..1, multiplied with the alpha
 * it has) and returns "#rrggbbaa". A colour that cannot be parsed is returned
 * unchanged.
 */
function withAlpha(color, alpha) {
  const c = parseColor(color);
  if (!c) return color;
  return `#${hex2(c.r)}${hex2(c.g)}${hex2(c.b)}${hex2(Math.min(1, Math.max(0, c.a * alpha)) * 255)}`;
}

/** The gutter dot of a level, as SVG text. */
function dotSvg(color, opacity) {
  const c = parseColor(color);
  const fill = c ? `#${hex2(c.r)}${hex2(c.g)}${hex2(c.b)}` : String(color).replace(/[^\w#(),.%\s-]/g, '');
  const o = Math.min(1, Math.max(0, (c ? c.a : 1) * opacity));
  return (
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16">' +
    `<circle cx="8" cy="8" r="4" fill="${fill}" fill-opacity="${Number(o.toFixed(3))}"/></svg>`
  );
}

/** A short, file-name-safe hash of a string. */
function hash(text) {
  let h = 5381;
  for (let i = 0; i < text.length; i++) h = ((h * 33) ^ text.charCodeAt(i)) >>> 0;
  return h.toString(16);
}

module.exports = { parseColor, withAlpha, dotSvg, hash };
