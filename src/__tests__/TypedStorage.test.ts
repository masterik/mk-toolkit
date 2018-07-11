import { TypedStorage } from '../typed-storage';
import { TypedKey, CopyConstructor } from '../types';

class TestClass {
  constructor(values?: Partial<TestClass>) {
    Object.assign(this, values || {});
  }
}

describe('setItem', () => {
  beforeEach(() => localStorage.clear());

  it('should store value to web storage', () => {
    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', 'test_value');

    expect(localStorage.setItem).toHaveBeenLastCalledWith('test_key', 'test_value');
    expect(localStorage.getItem('test_key')).toBe('test_value');
  });

  it('should store value by string key', () => {
    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', 'test_value');
    expect(localStorage['test_key']).toBe('test_value');
  });

  it('should store value by typed key', () => {
    const storage = new TypedStorage(localStorage);
    const typedKey = new TypedKey(TestClass, 'test_key');
    storage.setItem(typedKey, 'test_value');
    expect(localStorage['test_key']).toBe('test_value');
  });

  it.each(['', 'test', 0, 1, true, false, new Date() ])
  ('should store primitive value as string: %o', value => {
    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', value);
    expect(localStorage['test_key']).toBe(value.toString());
  });

  it.each([{}, new TestClass(), [], { foo: 'bar' }])
  ('should serialize other values to json: %o', value => {
    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', value);
    expect(localStorage['test_key']).toBe(JSON.stringify(value));
  });

  it.each([undefined, null])
  ('should not store non-serializable values: %o', value => {
    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', value);
    expect(localStorage['test_key']).toBe(undefined);
  });
});
