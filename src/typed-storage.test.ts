import { TypedStorage } from './typed-storage';
import { TypedKey } from './types';

class TestClass {
  readonly foo: string;

  constructor(values?: Partial<TestClass>) {
    Object.assign(this, values);
  }
}

describe('setItem', () => {
  beforeEach(() => localStorage.clear());

  it('should store item to web storage', () => {
    const storage = new TypedStorage(localStorage);

    storage.setItem('test_key', 'test_value');

    const expectedValue = JSON.stringify('test_value');
    expect(localStorage.setItem).toHaveBeenLastCalledWith('test_key', expectedValue);
    expect(localStorage.getItem('test_key')).toBe(expectedValue);
  });

  it('should replace item when exists', () => {
    localStorage.setItem('test_key', 'initial_value');
    expect(localStorage.getItem('test_key')).toBe('initial_value');

    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', 'test_value');

    const expectedValue = JSON.stringify('test_value');
    expect(localStorage.setItem).toHaveBeenLastCalledWith('test_key', expectedValue);
    expect(localStorage.getItem('test_key')).toBe(expectedValue);
  });

  it('should store value by string key', () => {
    const storage = new TypedStorage(localStorage);

    storage.setItem('test_key', 'test_value');

    const expectedValue = JSON.stringify('test_value');
    expect(localStorage.getItem('test_key')).toBe(expectedValue);
  });

  it('should store value by typed key', () => {
    const storage = new TypedStorage(localStorage);
    const typedKey = new TypedKey(TestClass, 'test_key');
    const value = new TestClass({ foo: 'test_value' });

    storage.setItem(typedKey, value);

    const expectedValue = JSON.stringify(value);
    expect(localStorage.getItem('test_key')).toBe(expectedValue);
  });

  it.each(['', 'test', 0, 1, true, false, new Date() ])('should serialize primitive values to json: %o', value => {
    const storage = new TypedStorage(localStorage);

    storage.setItem('test_key', value);

    const expectedValue = JSON.stringify(value);
    expect(localStorage.getItem('test_key')).toBe(expectedValue);
  });

  it.each([{}, new TestClass(), [], { foo: 'bar' }])('should serialize other values to json: %o', value => {
    const storage = new TypedStorage(localStorage);

    storage.setItem('test_key', value);

    const expectedValue = JSON.stringify(value);
    expect(localStorage.getItem('test_key')).toBe(expectedValue);
  });

  it.each([undefined, null])('should not store non-serializable values: %o', value => {
    const storage = new TypedStorage(localStorage);

    storage.setItem('test_key', value);

    expect(localStorage['test_key']).toBe(undefined);
  });
});


describe('getItem', () => {
  beforeEach(() => localStorage.clear());

  it('should get value from web storage', () => {
    localStorage.setItem('test_key', JSON.stringify('test_value'));

    const storage = new TypedStorage(localStorage);
    const value = storage.getItem('test_key');

    expect(localStorage.getItem).toHaveBeenLastCalledWith('test_key');
    expect(value).toBe('test_value');
  });

  it('should return null when not exists in storage', () => {
    const storage = new TypedStorage(localStorage);
    const value = storage.getItem('unknown');

    expect(value).toBe(null);
  });

  it('should return null when not json serialized', () => {
    localStorage.setItem('test_key', 'test_value');

    const storage = new TypedStorage(localStorage);
    const value = storage.getItem('test_key');

    expect(value).toBe(null);
  });

  it('should return value of type string', () => {
    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', 'test_value');

    const value: string = storage.getItem('test_key');

    expect(value).toBe('test_value');
    expect(typeof value).toBe('string');
  });

  it('should return value of type number', () => {
    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', 10);

    const value: number = storage.getItem('test_key');

    expect(value).toBe(10);
    expect(typeof value).toBe('number');
  });

  it('should return value of type boolean', () => {
    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', true);

    const value: boolean = storage.getItem('test_key');

    expect(value).toBe(true);
    expect(typeof value).toBe('boolean');
  });

  it('should return value of type Date', () => {
    const storage = new TypedStorage(localStorage);
    const typedKey = new TypedKey(Date, 'test_key');
    storage.setItem(typedKey, new Date(2018, 7, 11, 0, 0, 0));

    const value = storage.getItem(typedKey);

    expect(value).toEqual(new Date(2018, 7, 11, 0, 0, 0));
    expect(value instanceof Date).toBe(true);
  });

  it('should return value by typed key', () => {
    const storage = new TypedStorage(localStorage);
    const typedKey = new TypedKey(TestClass, 'test_key');
    const testValue = new TestClass({ foo: 'test_value' });
    storage.setItem(typedKey, testValue);

    const value = storage.getItem(typedKey);

    expect(value).toEqual(testValue);
    expect(value instanceof TestClass).toBe(true);
  });
});


describe('removeItem', () => {
  beforeEach(() => localStorage.clear());

  it('should remove item from web storage', () => {
    const storage = new TypedStorage(localStorage);
    storage.setItem('test_key', 'test_value');
    expect(localStorage.getItem('test_key')).toBeTruthy();

    storage.removeItem('test_key');

    expect(localStorage.removeItem).toHaveBeenLastCalledWith('test_key');
    expect(localStorage.getItem('test_key')).toBeFalsy();
  });

  it('should not throw when item not exists', () => {
    const storage = new TypedStorage(localStorage);
    expect(localStorage.getItem('test_key')).toBeFalsy();

    expect(() => storage.removeItem('test_key')).not.toThrow();
    expect(localStorage.removeItem).toHaveBeenLastCalledWith('test_key');
  });
});


describe('clear', () => {
  beforeEach(() => localStorage.clear());

  it('should clear web storage', () => {
    localStorage.setItem('test_key', 'test_value');
    expect(localStorage.getItem('test_key')).toBeTruthy();

    const storage = new TypedStorage(localStorage);
    const value = storage.clear();

    expect(localStorage.clear).toHaveBeenLastCalledWith();
    expect(localStorage.getItem('test_key')).toBeFalsy();
    expect(localStorage.length).toBe(0);
  });
});
