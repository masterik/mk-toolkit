import { isPrimitive } from '../helpers';

class TestClass { }
function testFunc() { /**/ }

describe('isPrimitive', () => {
  it.each(['', 'test', ' '])
  ('should return true for string value: %o', value => {
    expect(isPrimitive(value)).toBe(true);
  });

  it.each([0, 1, -10, 123.456])
  ('should return true for number value: %o', value => {
    expect(isPrimitive(value)).toBe(true);
  });

  it.each([true, false])
  ('should return true for boolean value: %o', value => {
    expect(isPrimitive(value)).toBe(true);
  });

  it('should return true for Date value', () => {
    expect(isPrimitive(new Date())).toBe(true);
  });

  it.each([undefined, null, {}, new TestClass(), [] ])
  ('should return false for other values: %o', value => {
    expect(isPrimitive(value)).toBe(false);
  });

  it('should return false for functions', () => {
    expect(isPrimitive(testFunc)).toBe(false);
    expect(isPrimitive(() => { /**/ })).toBe(false);
  });
});
