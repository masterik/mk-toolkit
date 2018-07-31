import { TypedKey, ArrayKey } from '../types';
import { resolveValue } from '../resolveValue';

describe('resolveValue', () => {
  test('should resolve value with correct prototype', () => {
    const typedKey = new TypedKey(TestClass, 'test');
    const value = resolveValue(typedKey, { foo: 'test', bar: 10 });

    expect(value).toBeTruthy();
    expect(value instanceof TestClass).toBe(true);
    expect(value.foo).toBe('test');
    expect(value.bar).toBe(10);
  });

  test('should resolve array with items having correct prototype', () => {
    const arrayKey = new ArrayKey(TestClass, 'test');
    const value = resolveValue(arrayKey, [{ foo: 'T1', bar: 1 }, { foo: 'T2', bar: 2 }]);

    expect(Array.isArray(value)).toBe(true);
    expect(value.length).toBe(2);
    expect(value[0].foo).toBe('T1');
    expect(value[0].bar).toBe(1);
    expect(value[1].foo).toBe('T2');
    expect(value[1].bar).toBe(2);
  });
});


class TestClass {
  readonly foo: string;
  readonly bar: number;
}
