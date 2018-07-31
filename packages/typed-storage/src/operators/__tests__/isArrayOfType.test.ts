import { isArrayOfType } from '../isArrayOfType';

class TestClass { /**/ }

describe('isArrayOfType', () => {
  test('should return true when all elements of specified type', () => {
    expect(isArrayOfType([1, 2], Number)).toBe(true);
    expect(isArrayOfType(['', 'test'], String)).toBe(true);
    expect(isArrayOfType([true, false], Boolean)).toBe(true);
    expect(isArrayOfType([new TestClass(), new TestClass()], TestClass)).toBe(true);
  });

  test('should return true for empty array', () => {
    const typedEmptyArr: TestClass[] = [];

    expect(isArrayOfType([], Number)).toBe(true);
    expect(isArrayOfType([], String)).toBe(true);
    expect(isArrayOfType([], Boolean)).toBe(true);
    expect(isArrayOfType([], TestClass)).toBe(true);
    expect(isArrayOfType(typedEmptyArr, TestClass)).toBe(true);
  });

  test('should return false when type mismatch', () => {
    expect(isArrayOfType([1, 2] as any[], String)).toBe(false);
    expect(isArrayOfType(['', 'test'] as any[], Number)).toBe(false);
    expect(isArrayOfType([1, 2], TestClass)).toBe(false);
    expect(isArrayOfType([new TestClass(), new TestClass()], String)).toBe(false);
  });

  test('should return false when mixed array (tuple)', () => {
    expect(isArrayOfType([1, 'test'] as any[], String)).toBe(false);
    expect(isArrayOfType([new TestClass(), {}], TestClass)).toBe(false);
  });
});
