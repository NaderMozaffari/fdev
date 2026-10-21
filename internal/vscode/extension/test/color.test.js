'use strict';

const test = require('node:test');
const assert = require('node:assert');
const { parseColor, withAlpha, dotSvg, hash } = require('../src/color');

test('parseColor reads hex and rgb', () => {
  assert.deepStrictEqual(parseColor('#58a6ff'), { r: 88, g: 166, b: 255, a: 1 });
  assert.deepStrictEqual(parseColor('#fff'), { r: 255, g: 255, b: 255, a: 1 });
  assert.strictEqual(parseColor('#00000080').a.toFixed(2), '0.50');
  assert.deepStrictEqual(parseColor('rgb(1, 2, 3)'), { r: 1, g: 2, b: 3, a: 1 });
  assert.deepStrictEqual(parseColor('rgba(1 2 3 / 50%)'), { r: 1, g: 2, b: 3, a: 0.5 });
  assert.strictEqual(parseColor('orange'), null);
  assert.strictEqual(parseColor(5), null);
});

test('withAlpha multiplies the alpha and keeps unknown colours', () => {
  assert.strictEqual(withAlpha('#ff0000', 0.5), '#ff000080');
  assert.strictEqual(withAlpha('#ff000080', 0.5), '#ff000040');
  assert.strictEqual(withAlpha('orange', 0.5), 'orange');
  assert.strictEqual(withAlpha('#ff0000', 5), '#ff0000ff');
});

test('dotSvg is valid and cannot be broken out of', () => {
  assert.match(dotSvg('#58a6ff', 0.35), /fill="#58a6ff" fill-opacity="0.35"/);
  assert.ok(!dotSvg('red" onload="x', 1).includes('onload="x'));
  assert.notStrictEqual(hash('a'), hash('b'));
});
